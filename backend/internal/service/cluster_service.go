// Package service holds the business logic that turns raw StarRocks metadata
// into the shapes the dashboard renders.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

// ErrUnavailable reports that StarRocks could not be reached or rejected the
// metadata query. The API layer turns it into 503 rather than 500, because the
// dashboard itself is healthy — the cluster it observes is not.
var ErrUnavailable = errors.New("starrocks unavailable")

// ErrInvalidArgument reports a caller mistake the API layer turns into 400.
var ErrInvalidArgument = errors.New("invalid argument")

// ErrNotFound reports a missing catalog object; the API layer turns it into 404.
var ErrNotFound = errors.New("not found")

// clusterReader is the slice of the repository this service depends on, declared
// here so the service can be tested against a fake.
type clusterReader interface {
	Frontends(ctx context.Context) ([]repository.Row, error)
	Backends(ctx context.Context) ([]repository.Row, error)
	ComputeNodes(ctx context.Context) ([]repository.Row, error)
	RunModeConfig(ctx context.Context) ([]repository.Row, error)
}

// ClusterService assembles the cluster topology view.
type ClusterService struct {
	repo clusterReader
	now  func() time.Time

	// policyGuard carries the runtime-tunable alert thresholds.
	policyGuard
}

// NewClusterService wires the service to a repository.
func NewClusterService(repo clusterReader) *ClusterService {
	return &ClusterService{repo: repo, now: time.Now}
}

// Topology reads FE, BE and CN membership and folds it into a single payload.
//
// FE and BE reads are required — both statements exist on every supported
// version. CN and run-mode reads are best-effort: SHOW COMPUTE NODES predates
// nothing on old releases and reading FE config needs ADMIN privilege, and a
// topology view must not go dark over either.
func (s *ClusterService) Topology(ctx context.Context) (model.Topology, error) {
	feRows, err := s.repo.Frontends(ctx)
	if err != nil {
		return model.Topology{}, fmt.Errorf("%w: SHOW FRONTENDS failed: %w", ErrUnavailable, err)
	}

	beRows, err := s.repo.Backends(ctx)
	if err != nil {
		return model.Topology{}, fmt.Errorf("%w: SHOW BACKENDS failed: %w", ErrUnavailable, err)
	}

	// Best-effort: an error means the version has no compute nodes.
	cnRows, err := s.repo.ComputeNodes(ctx)
	if err != nil {
		cnRows = nil
	}

	frontends := make([]model.Node, 0, len(feRows))
	for _, row := range feRows {
		frontends = append(frontends, mapFrontend(row))
	}
	sortFrontends(frontends)
	applyJournalLag(frontends)

	backends := make([]model.Node, 0, len(beRows))
	for _, row := range beRows {
		backends = append(backends, mapDataNode(row, model.NodeTypeBackend))
	}
	sortBackends(backends)

	computeNodes := make([]model.Node, 0, len(cnRows))
	for _, row := range cnRows {
		computeNodes = append(computeNodes, mapDataNode(row, model.NodeTypeCompute))
	}
	sortBackends(computeNodes)

	return model.Topology{
		CollectedAt:  s.now().UTC(),
		RunMode:      s.resolveRunMode(ctx),
		Summary:      summarize(frontends, backends, computeNodes),
		Frontends:    frontends,
		Backends:     backends,
		ComputeNodes: computeNodes,
	}, nil
}

// resolveRunMode reads the FE `run_mode` config. Three outcomes:
//   - a value ("shared_data" / "shared_nothing"): reported as-is;
//   - an empty result: the config item does not exist, which only happens on
//     pre-3.0 releases — those are always shared-nothing;
//   - an error (typically missing ADMIN privilege): honest "unknown".
func (s *ClusterService) resolveRunMode(ctx context.Context) string {
	rows, err := s.repo.RunModeConfig(ctx)
	if err != nil {
		return model.RunModeUnknown
	}

	for _, row := range rows {
		if !strings.EqualFold(row.Str("key"), "run_mode") {
			continue
		}
		switch value := strings.ToLower(row.Str("value")); value {
		case model.RunModeSharedData, model.RunModeSharedNothing:
			return value
		case "":
			return model.RunModeUnknown
		default:
			// Pass a future mode through rather than masking it.
			return value
		}
	}
	return model.RunModeSharedNothing
}

// mapFrontend converts one SHOW FRONTENDS row. Column names have drifted across
// StarRocks releases (IP/Host, Role/IsMaster), so each field lists its aliases.
func mapFrontend(row repository.Row) model.Node {
	name := row.Str("name")
	host := row.Str("ip", "host", "hostname")
	if name == "" {
		name = host
	}

	node := model.Node{
		ID:                "fe:" + firstNonEmpty(name, host, "unknown"),
		Name:              name,
		Type:              model.NodeTypeFrontend,
		Role:              frontendRole(row),
		Alive:             row.Bool("alive"),
		Host:              host,
		Ports:             collectPorts(row, "queryport", "httpport", "editlogport", "rpcport"),
		Version:           row.Str("version"),
		StartTime:         row.Str("starttime", "laststarttime"),
		LastHeartbeat:     row.Str("lastheartbeat"),
		ErrMsg:            row.Str("errmsg"),
		ReplayedJournalID: row.Int64("replayedjournalid", "replayed_journal_id"),
		IsHelper:          boolPtr(row, "ishelper", "is_helper"),
		Joined:            boolPtr(row, "join", "joined"),
		ClusterID:         row.Str("clusterid", "cluster_id"),
	}

	node.Status = model.StatusDown
	if node.Alive {
		node.Status = model.StatusHealthy
	}
	return node
}

// mapDataNode converts one SHOW BACKENDS or SHOW COMPUTE NODES row — the two
// statements share almost every column, including the capacity metrics that
// make skew and disk pressure visible.
func mapDataNode(row repository.Row, nodeType model.NodeType) model.Node {
	host := row.Str("ip", "host", "hostname")
	nodeID := row.Str("backendid", "computenodeid", "beid", "cnid", "id")

	idPrefix, role := "be:", model.RoleBackend
	if nodeType == model.NodeTypeCompute {
		idPrefix, role = "cn:", model.RoleCompute
	}

	node := model.Node{
		ID:                idPrefix + firstNonEmpty(nodeID, host, "unknown"),
		Name:              firstNonEmpty(host, string(nodeType)+"-"+nodeID),
		Type:              nodeType,
		Role:              role,
		Alive:             row.Bool("alive"),
		Host:              host,
		Ports:             collectPorts(row, "heartbeatport", "beport", "httpport", "brpcport", "starletport"),
		Version:           row.Str("version"),
		StartTime:         row.Str("laststarttime", "starttime"),
		LastHeartbeat:     row.Str("lastheartbeat"),
		ErrMsg:            row.Str("errmsg"),
		ReplayedJournalID: row.Int64("replayedjournalid", "replayed_journal_id"),
		IsHelper:          boolPtr(row, "ishelper", "is_helper"),
		Joined:            boolPtr(row, "join", "joined"),
		ClusterID:         row.Str("clusterid", "cluster_id"),
		Warehouse:         row.Str("warehousename", "warehouse"),

		TabletNum:       row.Int64("tabletnum"),
		CPUCores:        row.Int64("cpucores"),
		RunningQueries:  row.Int64("numrunningqueries"),
		DiskUsedPercent: row.Float64("maxdiskusedpct", "usedpct"),
		MemUsedPercent:  row.Float64("memusedpct"),
		CPUUsedPercent:  row.Float64("cpuusedpct"),
		DataUsedBytes:   row.Bytes("datausedcapacity"),
		TotalBytes:      row.Bytes("totalcapacity"),
		AvailableBytes:  row.Bytes("availcapacity"),
	}

	decommissioned := row.Bool("systemdecommissioned") || row.Bool("clusterdecommissioned")
	switch {
	case !node.Alive:
		node.Status = model.StatusDown
	case decommissioned:
		node.Status = model.StatusDecommissioned
	default:
		node.Status = model.StatusHealthy
	}
	return node
}

// frontendRole resolves the FE role. Older releases always report FOLLOWER and
// flag the elected node through a separate boolean column.
func frontendRole(row repository.Row) string {
	if row.Bool("ismaster", "isleader", "leader", "master") {
		return model.RoleLeader
	}

	switch role := strings.ToUpper(row.Str("role")); role {
	case "LEADER", "MASTER":
		return model.RoleLeader
	case model.RoleFollower, model.RoleObserver:
		return role
	case "":
		return model.RoleUnknown
	default:
		return role
	}
}

func collectPorts(row repository.Row, names ...string) map[string]int {
	ports := make(map[string]int, len(names))
	for _, name := range names {
		if v := row.Int64(name); v != nil && *v > 0 {
			ports[portLabels[name]] = int(*v)
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

// portLabels maps StarRocks column names to camelCase JSON keys.
var portLabels = map[string]string{
	"queryport":     "query",
	"httpport":      "http",
	"editlogport":   "editLog",
	"rpcport":       "rpc",
	"heartbeatport": "heartbeat",
	"beport":        "be",
	"brpcport":      "brpc",
	"starletport":   "starlet",
}

func summarize(frontends, backends, computeNodes []model.Node) model.TopologySummary {
	// Frontends must all report the same ClusterId; a second value means a node
	// was pointed at the wrong cluster.
	clusterIDs := make(map[string]struct{}, 1)
	summary := model.TopologySummary{
		FrontendTotal: len(frontends),
		BackendTotal:  len(backends),
		ComputeTotal:  len(computeNodes),
	}

	for _, fe := range frontends {
		if fe.Alive {
			summary.FrontendAlive++
		}
		if fe.Role == model.RoleLeader || fe.Role == model.RoleFollower {
			summary.ElectableTotal++
			if fe.Alive {
				summary.ElectableAlive++
			}
		}
		if fe.ClusterID != "" {
			clusterIDs[fe.ClusterID] = struct{}{}
		}
		// Only live replicas say anything useful about replication health; a
		// dead one is already reported as down.
		if fe.Alive && fe.Role != model.RoleLeader && fe.JournalLag != nil {
			if summary.MaxJournalLag == nil || *fe.JournalLag > *summary.MaxJournalLag {
				lag := *fe.JournalLag
				summary.MaxJournalLag = &lag
			}
		}
		if fe.Role == model.RoleLeader && fe.Alive {
			summary.LeaderHost = fe.Host
		}
	}

	for _, be := range backends {
		if be.Alive {
			summary.BackendAlive++
		}
		if be.TabletNum != nil {
			summary.TabletTotal += *be.TabletNum
		}
	}

	for _, cn := range computeNodes {
		if cn.Alive {
			summary.ComputeAlive++
		}
		if cn.TabletNum != nil {
			summary.TabletTotal += *cn.TabletNum
		}
	}

	summary.ClusterIDMismatch = len(clusterIDs) > 1
	// A metadata quorum needs a strict majority of the electable frontends;
	// below that, metadata writes block even though queries may still serve.
	summary.QuorumHealthy = summary.ElectableTotal > 0 &&
		summary.ElectableAlive > summary.ElectableTotal/2

	// Compute capacity may come from BEs (shared-nothing) or CNs (shared-data);
	// either satisfies the "can this cluster serve queries" requirement.
	summary.Healthy = summary.LeaderHost != "" &&
		summary.QuorumHealthy &&
		!summary.ClusterIDMismatch &&
		summary.FrontendAlive == summary.FrontendTotal &&
		summary.BackendAlive == summary.BackendTotal &&
		summary.ComputeAlive == summary.ComputeTotal &&
		summary.BackendTotal+summary.ComputeTotal > 0

	return summary
}

// sortFrontends puts the leader first, then followers, then observers, so the
// most operationally relevant node is always at the top of the list.
func sortFrontends(nodes []model.Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if ri, rj := roleRank(nodes[i].Role), roleRank(nodes[j].Role); ri != rj {
			return ri < rj
		}
		return nodes[i].Name < nodes[j].Name
	})
}

// sortBackends orders by host so the list stays stable across polls even as
// backend ids are recycled.
func sortBackends(nodes []model.Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Host != nodes[j].Host {
			return nodes[i].Host < nodes[j].Host
		}
		return nodes[i].ID < nodes[j].ID
	})
}

func roleRank(role string) int {
	switch role {
	case model.RoleLeader:
		return 0
	case model.RoleFollower:
		return 1
	case model.RoleObserver:
		return 2
	default:
		return 3
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// boolPtr reports a tri-state boolean column: nil when the running StarRocks
// version does not report it, so "false" never masquerades as "not reported".
func boolPtr(row repository.Row, names ...string) *bool {
	if row.Str(names...) == "" {
		return nil
	}
	v := row.Bool(names...)
	return &v
}

// applyJournalLag fills JournalLag on every frontend relative to the elected
// leader's replay position.
//
// Replication lag is the difference between the leader's journal id and each
// replica's: the leader is 0 by definition, and a follower that stops
// advancing is unable to take over even while it still answers heartbeats —
// which is exactly the failure Alive alone cannot show.
func applyJournalLag(frontends []model.Node) {
	var leaderJournal *int64
	for _, fe := range frontends {
		if fe.Role == model.RoleLeader && fe.Alive && fe.ReplayedJournalID != nil {
			leaderJournal = fe.ReplayedJournalID
			break
		}
	}
	if leaderJournal == nil {
		return
	}

	for i := range frontends {
		journal := frontends[i].ReplayedJournalID
		if journal == nil {
			continue
		}
		// A replica briefly ahead of the leader's snapshot reads as caught up
		// rather than as negative lag.
		lag := max(*leaderJournal-*journal, 0)
		frontends[i].JournalLag = &lag
	}
}
