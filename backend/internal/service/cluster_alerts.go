package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
	"github.com/sangyun-han/StarLens/backend/internal/model"
)

// Cluster alert rule identifiers. Stable strings: they appear in webhook
// payloads and drive deduplication keys.
const (
	RuleClusterNodeDown          = "cluster_node_down"
	RuleClusterNoLeader          = "cluster_no_leader"
	RuleClusterQuorumLost        = "cluster_quorum_lost"
	RuleClusterIDMismatch        = "cluster_id_mismatch"
	RuleFrontendJournalLag       = "fe_journal_lag"
	RuleClusterNoComputeCapacity = "cluster_no_compute_capacity"
)

// ClusterAlertPolicy holds the thresholds the cluster rules evaluate against.
type ClusterAlertPolicy struct {
	// MaxJournalLag fires when a live frontend trails the leader's metadata
	// journal by more than this many entries. <= 0 disables the rule.
	MaxJournalLag int64
}

// SetAlertPolicy replaces the cluster alert thresholds at runtime.
func (s *ClusterService) SetAlertPolicy(policy ClusterAlertPolicy) {
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	s.policy = policy
}

func (s *ClusterService) currentPolicy() ClusterAlertPolicy {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	return s.policy
}

// policyGuard is embedded by ClusterService; kept here so the alerting concern
// stays in one file.
type policyGuard struct {
	policyMu sync.RWMutex
	policy   ClusterAlertPolicy
}

// CollectAlerts takes a fresh topology snapshot and evaluates every cluster
// rule — the alert.CollectFunc used by the background poller.
func (s *ClusterService) CollectAlerts(ctx context.Context) ([]alert.Alert, error) {
	topology, err := s.Topology(ctx)
	if err != nil {
		return nil, err
	}
	return s.EvaluateAlerts(topology), nil
}

// EvaluateAlerts applies the cluster rules to a topology snapshot. Pure
// function of its inputs: repeat suppression is the Manager's job.
func (s *ClusterService) EvaluateAlerts(topology model.Topology) []alert.Alert {
	policy := s.currentPolicy()
	summary := topology.Summary
	var alerts []alert.Alert

	// A cluster with no elected leader cannot accept metadata writes at all —
	// no DDL, no load bookkeeping, no schema change.
	if summary.LeaderHost == "" {
		alerts = append(alerts, alert.Alert{
			Key:      RuleClusterNoLeader,
			RuleID:   RuleClusterNoLeader,
			Severity: alert.SeverityCritical,
			Title:    "No frontend leader is elected",
			Message: fmt.Sprintf(
				"%d of %d electable frontends are alive. Metadata writes (DDL, load bookkeeping) are blocked until an election succeeds.",
				summary.ElectableAlive, summary.ElectableTotal),
		})
	} else if !summary.QuorumHealthy {
		// A leader without a majority behind it is living on borrowed time.
		alerts = append(alerts, alert.Alert{
			Key:      RuleClusterQuorumLost,
			RuleID:   RuleClusterQuorumLost,
			Severity: alert.SeverityCritical,
			Title:    "Frontend metadata quorum is lost",
			Message: fmt.Sprintf(
				"Only %d of %d electable frontends are alive; a majority is required. Restore a follower before the current leader restarts.",
				summary.ElectableAlive, summary.ElectableTotal),
			Labels: map[string]string{"leader": summary.LeaderHost},
		})
	}

	if summary.ClusterIDMismatch {
		alerts = append(alerts, alert.Alert{
			Key:      RuleClusterIDMismatch,
			RuleID:   RuleClusterIDMismatch,
			Severity: alert.SeverityCritical,
			Title:    "Frontends disagree on cluster id",
			Message:  "At least one frontend reports a different ClusterId, which means it belongs to another cluster. Remove it before it corrupts metadata.",
		})
	}

	// Compute capacity comes from BEs (shared-nothing) or CNs (shared-data);
	// with neither, the cluster cannot answer a query at all.
	if summary.BackendTotal+summary.ComputeTotal > 0 &&
		summary.BackendAlive+summary.ComputeAlive == 0 {
		alerts = append(alerts, alert.Alert{
			Key:      RuleClusterNoComputeCapacity,
			RuleID:   RuleClusterNoComputeCapacity,
			Severity: alert.SeverityCritical,
			Title:    "No backend or compute node is alive",
			Message:  "Every storage/compute node is down; queries cannot be served.",
		})
	}

	alerts = append(alerts, nodeDownAlerts(topology)...)
	alerts = append(alerts, journalLagAlerts(policy, topology.Frontends)...)
	return alerts
}

// nodeDownAlerts reports one alert per dead node, keyed by node id so a
// recurring outage on one host does not suppress a second host going down.
func nodeDownAlerts(topology model.Topology) []alert.Alert {
	var alerts []alert.Alert

	for _, group := range [][]model.Node{topology.Frontends, topology.Backends, topology.ComputeNodes} {
		for _, node := range group {
			if node.Alive {
				continue
			}
			labels := map[string]string{
				"node": node.Name,
				"host": node.Host,
				"type": string(node.Type),
				"role": node.Role,
			}
			message := fmt.Sprintf("%s node %s (%s) stopped answering heartbeats.",
				node.Type, node.Name, node.Host)
			if node.ErrMsg != "" {
				message += " StarRocks reports: " + node.ErrMsg
			}
			if node.LastHeartbeat != "" {
				message += fmt.Sprintf(" Last heartbeat: %s.", node.LastHeartbeat)
			}

			alerts = append(alerts, alert.Alert{
				Key:      RuleClusterNodeDown + "|" + node.ID,
				RuleID:   RuleClusterNodeDown,
				Severity: alert.SeverityCritical,
				Title:    fmt.Sprintf("%s node %q is down", node.Type, nodeSubject(node)),
				Message:  message,
				Labels:   labels,
			})
		}
	}
	return alerts
}

// journalLagAlerts reports frontends whose metadata replication trails the
// leader. Such a node answers heartbeats but cannot take over cleanly, so it
// is invisible to a liveness-only check.
func journalLagAlerts(policy ClusterAlertPolicy, frontends []model.Node) []alert.Alert {
	if policy.MaxJournalLag <= 0 {
		return nil
	}

	var alerts []alert.Alert
	for _, fe := range frontends {
		if !fe.Alive || fe.Role == model.RoleLeader || fe.JournalLag == nil {
			continue
		}
		if *fe.JournalLag <= policy.MaxJournalLag {
			continue
		}

		alerts = append(alerts, alert.Alert{
			Key:      RuleFrontendJournalLag + "|" + fe.ID,
			RuleID:   RuleFrontendJournalLag,
			Severity: alert.SeverityWarning,
			Title: fmt.Sprintf("Frontend %q trails the leader by %d journal entries",
				nodeSubject(fe), *fe.JournalLag),
			Message: fmt.Sprintf(
				"Metadata replication is behind by %d entries (threshold %d). This %s cannot take over cleanly until it catches up.",
				*fe.JournalLag, policy.MaxJournalLag, strings.ToLower(fe.Role)),
			Labels: map[string]string{
				"node": fe.Name,
				"host": fe.Host,
				"role": fe.Role,
			},
		})
	}
	return alerts
}

// nodeSubject prefers the node name and falls back to its host.
func nodeSubject(node model.Node) string {
	if node.Name != "" {
		return node.Name
	}
	return node.Host
}
