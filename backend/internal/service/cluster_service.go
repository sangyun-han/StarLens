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
		ID:            "fe:" + firstNonEmpty(name, host, "unknown"),
		Name:          name,
		Type:          model.NodeTypeFrontend,
		Role:          frontendRole(row),
		Alive:         row.Bool("alive"),
		Host:          host,
		Ports:         collectPorts(row, "queryport", "httpport", "editlogport", "rpcport"),
		Version:       row.Str("version"),
		StartTime:     row.Str("starttime", "laststarttime"),
		LastHeartbeat: row.Str("lastheartbeat"),
		ErrMsg:        row.Str("errmsg"),
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
		ID:            idPrefix + firstNonEmpty(nodeID, host, "unknown"),
		Name:          firstNonEmpty(host, string(nodeType)+"-"+nodeID),
		Type:          nodeType,
		Role:          role,
		Alive:         row.Bool("alive"),
		Host:          host,
		Ports:         collectPorts(row, "heartbeatport", "beport", "httpport", "brpcport", "starletport"),
		Version:       row.Str("version"),
		StartTime:     row.Str("laststarttime", "starttime"),
		LastHeartbeat: row.Str("lastheartbeat"),
		ErrMsg:        row.Str("errmsg"),
		Warehouse:     row.Str("warehousename", "warehouse"),

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
	summary := model.TopologySummary{
		FrontendTotal: len(frontends),
		BackendTotal:  len(backends),
		ComputeTotal:  len(computeNodes),
	}

	for _, fe := range frontends {
		if fe.Alive {
			summary.FrontendAlive++
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

	// Compute capacity may come from BEs (shared-nothing) or CNs (shared-data);
	// either satisfies the "can this cluster serve queries" requirement.
	summary.Healthy = summary.LeaderHost != "" &&
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
