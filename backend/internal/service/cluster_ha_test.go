package service

import (
	"context"
	"strings"
	"testing"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

// A three-frontend HA setup: an elected leader, a follower that has fallen
// behind on metadata replication, and a read-only observer.
func haRepo() fakeClusterRepo {
	return fakeClusterRepo{
		frontends: []repository.Row{
			{
				"name": "fe_leader", "ip": "10.0.0.1", "role": "LEADER", "alive": "true",
				"replayedjournalid": "10000", "ishelper": "true", "join": "true",
				"clusterid": "1405478932",
			},
			{
				"name": "fe_follower", "ip": "10.0.0.2", "role": "FOLLOWER", "alive": "true",
				"replayedjournalid": "8500", "ishelper": "true", "join": "true",
				"clusterid": "1405478932",
			},
			{
				"name": "fe_observer", "ip": "10.0.0.3", "role": "OBSERVER", "alive": "true",
				"replayedjournalid": "9999", "ishelper": "false", "join": "true",
				"clusterid": "1405478932",
			},
		},
		backends: []repository.Row{{"backendid": "10001", "ip": "10.0.1.1", "alive": "true", "tabletnum": "100"}},
	}
}

func TestTopologyJournalLag(t *testing.T) {
	topology, err := NewClusterService(haRepo()).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}

	byName := map[string]model.Node{}
	for _, fe := range topology.Frontends {
		byName[fe.Name] = fe
	}

	// The leader defines the reference point, so its own lag is zero.
	if leader := byName["fe_leader"]; leader.JournalLag == nil || *leader.JournalLag != 0 {
		t.Errorf("leader JournalLag = %v, want 0", leader.JournalLag)
	}
	if follower := byName["fe_follower"]; follower.JournalLag == nil || *follower.JournalLag != 1500 {
		t.Errorf("follower JournalLag = %v, want 1500", follower.JournalLag)
	}
	if observer := byName["fe_observer"]; observer.JournalLag == nil || *observer.JournalLag != 1 {
		t.Errorf("observer JournalLag = %v, want 1", observer.JournalLag)
	}

	// The HA columns must round-trip, including the tri-state booleans.
	leader := byName["fe_leader"]
	if leader.ReplayedJournalID == nil || *leader.ReplayedJournalID != 10000 {
		t.Errorf("ReplayedJournalID = %v", leader.ReplayedJournalID)
	}
	if leader.IsHelper == nil || !*leader.IsHelper || leader.Joined == nil || !*leader.Joined {
		t.Errorf("IsHelper/Joined = %v/%v", leader.IsHelper, leader.Joined)
	}
	if leader.ClusterID != "1405478932" {
		t.Errorf("ClusterID = %q", leader.ClusterID)
	}

	// The summary reports the worst lag, and the observer is excluded from it
	// only if it is not a replica — it is one, so 1500 (the follower) wins.
	if topology.Summary.MaxJournalLag == nil || *topology.Summary.MaxJournalLag != 1500 {
		t.Errorf("MaxJournalLag = %v, want 1500", topology.Summary.MaxJournalLag)
	}
}

func TestTopologyMissingJournalColumnStaysNil(t *testing.T) {
	repo := fakeClusterRepo{
		frontends: []repository.Row{{"name": "fe1", "ip": "10.0.0.1", "role": "LEADER", "alive": "true"}},
		backends:  []repository.Row{{"backendid": "1", "ip": "10.0.1.1", "alive": "true"}},
	}

	topology, _ := NewClusterService(repo).Topology(context.Background())
	fe := topology.Frontends[0]
	if fe.ReplayedJournalID != nil || fe.JournalLag != nil || fe.IsHelper != nil || fe.Joined != nil {
		t.Errorf("absent columns must stay nil, got %+v", fe)
	}
	if topology.Summary.MaxJournalLag != nil {
		t.Error("MaxJournalLag must be nil when no frontend reports a journal position")
	}
}

func TestTopologyQuorum(t *testing.T) {
	cases := []struct {
		name       string
		frontends  []repository.Row
		wantAlive  int
		wantTotal  int
		wantQuorum bool
	}{
		{
			name: "leader plus two followers, one down",
			frontends: []repository.Row{
				{"name": "a", "ip": "1", "role": "LEADER", "alive": "true"},
				{"name": "b", "ip": "2", "role": "FOLLOWER", "alive": "true"},
				{"name": "c", "ip": "3", "role": "FOLLOWER", "alive": "false"},
			},
			wantAlive: 2, wantTotal: 3, wantQuorum: true,
		},
		{
			name: "two of three electable down loses the majority",
			frontends: []repository.Row{
				{"name": "a", "ip": "1", "role": "LEADER", "alive": "true"},
				{"name": "b", "ip": "2", "role": "FOLLOWER", "alive": "false"},
				{"name": "c", "ip": "3", "role": "FOLLOWER", "alive": "false"},
			},
			wantAlive: 1, wantTotal: 3, wantQuorum: false,
		},
		{
			name: "observers never count toward the quorum",
			frontends: []repository.Row{
				{"name": "a", "ip": "1", "role": "LEADER", "alive": "true"},
				{"name": "o1", "ip": "2", "role": "OBSERVER", "alive": "true"},
				{"name": "o2", "ip": "3", "role": "OBSERVER", "alive": "true"},
			},
			wantAlive: 1, wantTotal: 1, wantQuorum: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := fakeClusterRepo{
				frontends: tc.frontends,
				backends:  []repository.Row{{"backendid": "1", "ip": "10.0.1.1", "alive": "true"}},
			}
			topology, _ := NewClusterService(repo).Topology(context.Background())
			s := topology.Summary

			if s.ElectableAlive != tc.wantAlive || s.ElectableTotal != tc.wantTotal {
				t.Errorf("electable = %d/%d, want %d/%d", s.ElectableAlive, s.ElectableTotal, tc.wantAlive, tc.wantTotal)
			}
			if s.QuorumHealthy != tc.wantQuorum {
				t.Errorf("QuorumHealthy = %v, want %v", s.QuorumHealthy, tc.wantQuorum)
			}
			// Losing quorum must drag the overall verdict down with it.
			if !tc.wantQuorum && s.Healthy {
				t.Error("a cluster without quorum must not report Healthy")
			}
		})
	}
}

func TestTopologyClusterIDMismatch(t *testing.T) {
	repo := fakeClusterRepo{
		frontends: []repository.Row{
			{"name": "a", "ip": "1", "role": "LEADER", "alive": "true", "clusterid": "111"},
			{"name": "b", "ip": "2", "role": "FOLLOWER", "alive": "true", "clusterid": "222"},
		},
		backends: []repository.Row{{"backendid": "1", "ip": "10.0.1.1", "alive": "true"}},
	}

	topology, _ := NewClusterService(repo).Topology(context.Background())
	if !topology.Summary.ClusterIDMismatch {
		t.Error("frontends reporting different cluster ids must be flagged")
	}
	if topology.Summary.Healthy {
		t.Error("a cluster-id mismatch must not report Healthy")
	}
}

func TestClusterAlerts(t *testing.T) {
	svc := NewClusterService(haRepo())
	svc.SetAlertPolicy(ClusterAlertPolicy{MaxJournalLag: 1000})

	topology, _ := svc.Topology(context.Background())
	alerts := svc.EvaluateAlerts(topology)

	byRule := map[string]alert.Alert{}
	for _, a := range alerts {
		byRule[a.RuleID] = a
	}

	// The follower is 1500 behind, over the 1000 threshold.
	lag, ok := byRule[RuleFrontendJournalLag]
	if !ok {
		t.Fatalf("journal lag rule did not fire: %+v", alerts)
	}
	if lag.Labels["node"] != "fe_follower" || lag.Severity != alert.SeverityWarning {
		t.Errorf("lag alert = %+v", lag)
	}

	// Everything else in this fixture is healthy.
	for _, rule := range []string{RuleClusterNoLeader, RuleClusterQuorumLost, RuleClusterNodeDown, RuleClusterIDMismatch} {
		if _, fired := byRule[rule]; fired {
			t.Errorf("%s fired on a healthy cluster", rule)
		}
	}
}

func TestClusterAlertsNodeDownAndNoLeader(t *testing.T) {
	repo := fakeClusterRepo{
		frontends: []repository.Row{
			{"name": "a", "ip": "10.0.0.1", "role": "FOLLOWER", "alive": "true"},
			{"name": "b", "ip": "10.0.0.2", "role": "FOLLOWER", "alive": "false", "errmsg": "heartbeat timeout"},
		},
		backends: []repository.Row{{"backendid": "1", "ip": "10.0.1.1", "alive": "false"}},
	}
	svc := NewClusterService(repo)

	topology, _ := svc.Topology(context.Background())
	alerts := svc.EvaluateAlerts(topology)

	counts := map[string]int{}
	for _, a := range alerts {
		counts[a.RuleID]++
	}

	if counts[RuleClusterNoLeader] != 1 {
		t.Errorf("no-leader rule fired %d times, want 1", counts[RuleClusterNoLeader])
	}
	// One dead frontend and one dead backend, keyed per node.
	if counts[RuleClusterNodeDown] != 2 {
		t.Errorf("node-down fired %d times, want 2 (one per dead node)", counts[RuleClusterNodeDown])
	}
	// Every backend is down, so the cluster cannot serve queries either.
	if counts[RuleClusterNoComputeCapacity] != 1 {
		t.Errorf("no-compute-capacity fired %d times, want 1", counts[RuleClusterNoComputeCapacity])
	}

	// Dead-node alerts must carry the cause an operator needs.
	for _, a := range alerts {
		if a.RuleID == RuleClusterNodeDown && a.Labels["node"] == "b" {
			if !strings.Contains(a.Message, "heartbeat timeout") {
				t.Errorf("node-down message dropped the StarRocks reason: %q", a.Message)
			}
		}
	}
}

func TestClusterAlertsJournalLagDisabled(t *testing.T) {
	svc := NewClusterService(haRepo())
	svc.SetAlertPolicy(ClusterAlertPolicy{MaxJournalLag: 0}) // disabled

	topology, _ := svc.Topology(context.Background())
	for _, a := range svc.EvaluateAlerts(topology) {
		if a.RuleID == RuleFrontendJournalLag {
			t.Error("journal lag rule fired while disabled")
		}
	}
}
