package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

// fakeClusterRepo returns canned SHOW output. Keys are lower-cased because that
// is how repository.QueryRows normalizes column names.
type fakeClusterRepo struct {
	frontends    []repository.Row
	backends     []repository.Row
	computeNodes []repository.Row
	runMode      []repository.Row
	err          error
	// computeErr simulates a StarRocks version without SHOW COMPUTE NODES.
	computeErr error
	// runModeErr simulates a user without ADMIN privilege.
	runModeErr error
}

func (f fakeClusterRepo) Frontends(context.Context) ([]repository.Row, error) {
	return f.frontends, f.err
}

func (f fakeClusterRepo) Backends(context.Context) ([]repository.Row, error) {
	return f.backends, f.err
}

func (f fakeClusterRepo) ComputeNodes(context.Context) ([]repository.Row, error) {
	if f.computeErr != nil {
		return nil, f.computeErr
	}
	return f.computeNodes, f.err
}

func (f fakeClusterRepo) RunModeConfig(context.Context) ([]repository.Row, error) {
	if f.runModeErr != nil {
		return nil, f.runModeErr
	}
	return f.runMode, nil
}

// Column names mirror StarRocks 3.x `SHOW FRONTENDS` / `SHOW BACKENDS`.
func sampleRepo() fakeClusterRepo {
	return fakeClusterRepo{
		frontends: []repository.Row{
			{
				"name": "fe_observer", "ip": "172.26.92.3", "queryport": "9030", "httpport": "8030",
				"role": "OBSERVER", "alive": "false", "version": "3.3.5-abc",
				"lastheartbeat": "2026-08-17 09:59:00", "errmsg": "heartbeat timeout",
			},
			{
				"name": "fe_leader", "ip": "172.26.92.1", "queryport": "9030", "httpport": "8030",
				"editlogport": "9010", "rpcport": "9020",
				// Older releases report FOLLOWER plus a separate election flag.
				"role": "FOLLOWER", "ismaster": "true", "alive": "true",
				"starttime": "2026-08-01 09:00:00", "version": "3.3.5-abc",
			},
			{
				"name": "fe_follower", "ip": "172.26.92.2", "queryport": "9030",
				"role": "FOLLOWER", "ismaster": "false", "alive": "true", "version": "3.3.5-abc",
			},
		},
		backends: []repository.Row{
			{
				"backendid": "10002", "ip": "172.26.92.11", "heartbeatport": "9050", "beport": "9060",
				"httpport": "8040", "brpcport": "8060", "alive": "true",
				"systemdecommissioned": "false", "clusterdecommissioned": "false",
				"tabletnum": "1024", "datausedcapacity": "1.500 GB", "availcapacity": "98.500 GB",
				"totalcapacity": "100.000 GB", "maxdiskusedpct": "1.50 %", "usedpct": "1.50 %",
				"cpucores": "8", "numrunningqueries": "2", "memusedpct": "12.30 %", "cpuusedpct": "3.40 %",
				"version": "3.3.5-abc", "laststarttime": "2026-08-01 09:01:00",
			},
			{
				"backendid": "10003", "ip": "172.26.92.12", "heartbeatport": "9050", "alive": "true",
				"systemdecommissioned": "true", "clusterdecommissioned": "false",
				"tabletnum": "512", "version": "3.3.5-abc",
			},
			{
				"backendid": "10004", "ip": "172.26.92.13", "heartbeatport": "9050", "alive": "false",
				"tabletnum": "0", "errmsg": "backend heartbeat failed", "version": "3.3.5-abc",
			},
		},
	}
}

func TestTopologyMapsFrontends(t *testing.T) {
	topology, err := NewClusterService(sampleRepo()).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}

	if len(topology.Frontends) != 3 {
		t.Fatalf("frontends = %d, want 3", len(topology.Frontends))
	}

	// The elected leader must sort first regardless of input order.
	leader := topology.Frontends[0]
	if leader.Name != "fe_leader" {
		t.Errorf("frontends[0].Name = %q, want fe_leader", leader.Name)
	}
	if leader.Role != model.RoleLeader {
		t.Errorf("leader.Role = %q, want %q (IsMaster=true must win over Role=FOLLOWER)", leader.Role, model.RoleLeader)
	}
	if leader.ID != "fe:fe_leader" {
		t.Errorf("leader.ID = %q, want fe:fe_leader", leader.ID)
	}
	if leader.Host != "172.26.92.1" || leader.Type != model.NodeTypeFrontend {
		t.Errorf("leader host/type = %q/%q", leader.Host, leader.Type)
	}
	if leader.Status != model.StatusHealthy {
		t.Errorf("leader.Status = %q, want HEALTHY", leader.Status)
	}
	if got, want := leader.Ports["query"], 9030; got != want {
		t.Errorf("leader.Ports[query] = %d, want %d", got, want)
	}

	if got := topology.Frontends[1].Role; got != model.RoleFollower {
		t.Errorf("frontends[1].Role = %q, want FOLLOWER", got)
	}

	observer := topology.Frontends[2]
	if observer.Role != model.RoleObserver {
		t.Errorf("frontends[2].Role = %q, want OBSERVER", observer.Role)
	}
	if observer.Alive || observer.Status != model.StatusDown {
		t.Errorf("dead observer alive/status = %v/%q, want false/DOWN", observer.Alive, observer.Status)
	}
	if observer.ErrMsg != "heartbeat timeout" {
		t.Errorf("observer.ErrMsg = %q", observer.ErrMsg)
	}
}

func TestTopologyMapsBackends(t *testing.T) {
	topology, err := NewClusterService(sampleRepo()).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}

	if len(topology.Backends) != 3 {
		t.Fatalf("backends = %d, want 3", len(topology.Backends))
	}

	be := topology.Backends[0]
	if be.ID != "be:10002" || be.Name != "172.26.92.11" {
		t.Errorf("backends[0] id/name = %q/%q", be.ID, be.Name)
	}
	if be.Role != model.RoleBackend {
		t.Errorf("backends[0].Role = %q, want BACKEND", be.Role)
	}
	if be.TabletNum == nil || *be.TabletNum != 1024 {
		t.Errorf("backends[0].TabletNum = %v, want 1024", be.TabletNum)
	}
	// "1.500 GB" must parse into bytes for the frontend to format it.
	if be.DataUsedBytes == nil || *be.DataUsedBytes != int64(1.5*(1<<30)) {
		t.Errorf("backends[0].DataUsedBytes = %v, want %d", be.DataUsedBytes, int64(1.5*(1<<30)))
	}
	// "1.50 %" must parse as 1.5, not be rescaled.
	if be.DiskUsedPercent == nil || *be.DiskUsedPercent != 1.5 {
		t.Errorf("backends[0].DiskUsedPercent = %v, want 1.5", be.DiskUsedPercent)
	}
	if be.MemUsedPercent == nil || *be.MemUsedPercent != 12.3 {
		t.Errorf("backends[0].MemUsedPercent = %v, want 12.3", be.MemUsedPercent)
	}
	if be.CPUCores == nil || *be.CPUCores != 8 {
		t.Errorf("backends[0].CPUCores = %v, want 8", be.CPUCores)
	}

	// A draining node is alive but must not read as plain HEALTHY.
	if got := topology.Backends[1].Status; got != model.StatusDecommissioned {
		t.Errorf("backends[1].Status = %q, want DECOMMISSIONED", got)
	}
	if got := topology.Backends[2].Status; got != model.StatusDown {
		t.Errorf("backends[2].Status = %q, want DOWN", got)
	}

	// Versions that omit a column must yield nil, not a zero value.
	if topology.Backends[2].CPUCores != nil {
		t.Errorf("missing CpuCores column should map to nil, got %v", topology.Backends[2].CPUCores)
	}
}

func TestTopologySummary(t *testing.T) {
	topology, err := NewClusterService(sampleRepo()).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}

	got := topology.Summary
	want := model.TopologySummary{
		FrontendTotal: 3, FrontendAlive: 2,
		BackendTotal: 3, BackendAlive: 2,
		LeaderHost: "172.26.92.1", TabletTotal: 1536, Healthy: false,
	}
	if got != want {
		t.Errorf("Summary = %+v, want %+v", got, want)
	}

	if topology.CollectedAt.IsZero() {
		t.Error("CollectedAt must be stamped")
	}
}

func TestTopologySummaryHealthyCluster(t *testing.T) {
	repo := fakeClusterRepo{
		frontends: []repository.Row{{"name": "fe1", "ip": "10.0.0.1", "role": "LEADER", "alive": "true"}},
		backends:  []repository.Row{{"backendid": "10001", "ip": "10.0.0.2", "alive": "true", "tabletnum": "10"}},
	}

	topology, err := NewClusterService(repo).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}
	if !topology.Summary.Healthy {
		t.Errorf("Summary.Healthy = false, want true for %+v", topology.Summary)
	}
}

func TestTopologyNoLeaderIsNotHealthy(t *testing.T) {
	repo := fakeClusterRepo{
		frontends: []repository.Row{{"name": "fe1", "ip": "10.0.0.1", "role": "FOLLOWER", "alive": "true"}},
		backends:  []repository.Row{{"backendid": "10001", "ip": "10.0.0.2", "alive": "true"}},
	}

	topology, err := NewClusterService(repo).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}
	if topology.Summary.Healthy {
		t.Error("a cluster with no elected leader must not report Healthy")
	}
	if topology.Summary.LeaderHost != "" {
		t.Errorf("LeaderHost = %q, want empty", topology.Summary.LeaderHost)
	}
}

func TestTopologyWrapsRepositoryError(t *testing.T) {
	repo := fakeClusterRepo{err: errors.New("dial tcp 127.0.0.1:9030: connect: connection refused")}

	_, err := NewClusterService(repo).Topology(context.Background())
	if err == nil {
		t.Fatal("Topology() error = nil, want failure")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("error %v does not wrap ErrUnavailable", err)
	}
}

func TestTopologyEmptyClusterReturnsEmptySlices(t *testing.T) {
	topology, err := NewClusterService(fakeClusterRepo{}).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}
	// Non-nil slices keep the JSON as [] rather than null, which the frontend
	// would otherwise have to guard against.
	if topology.Frontends == nil || topology.Backends == nil || topology.ComputeNodes == nil {
		t.Error("node slices must be non-nil so they serialize as []")
	}
}

// A shared-data cluster: FE leader + CNs, zero BEs. Columns mirror
// SHOW COMPUTE NODES on StarRocks 3.x.
func sharedDataRepo() fakeClusterRepo {
	return fakeClusterRepo{
		frontends: []repository.Row{{
			"name": "fe1", "ip": "10.0.0.1", "role": "LEADER", "alive": "true",
		}},
		backends: nil, // SHOW BACKENDS returns no rows in shared-data mode
		computeNodes: []repository.Row{
			{
				"computenodeid": "50001", "ip": "10.0.1.1", "heartbeatport": "9050",
				"httpport": "8040", "starletport": "9070", "alive": "true",
				"cpucores": "16", "numrunningqueries": "4", "memusedpct": "35.20 %",
				"cpuusedpct": "12.10 %", "warehousename": "default_warehouse",
				"tabletnum": "256", "version": "3.3.5-abc",
			},
			{
				"computenodeid": "50002", "ip": "10.0.1.2", "heartbeatport": "9050",
				"alive": "false", "errmsg": "heartbeat timeout",
			},
		},
		runMode: []repository.Row{{
			"key": "run_mode", "value": "shared_data", "type": "String", "ismutable": "false",
		}},
	}
}

func TestTopologySharedDataCluster(t *testing.T) {
	topology, err := NewClusterService(sharedDataRepo()).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}

	if topology.RunMode != model.RunModeSharedData {
		t.Errorf("RunMode = %q, want shared_data", topology.RunMode)
	}

	if len(topology.ComputeNodes) != 2 {
		t.Fatalf("computeNodes = %d, want 2", len(topology.ComputeNodes))
	}
	cn := topology.ComputeNodes[0]
	if cn.ID != "cn:50001" || cn.Type != model.NodeTypeCompute || cn.Role != model.RoleCompute {
		t.Errorf("cn identity = %+v", cn)
	}
	if cn.Warehouse != "default_warehouse" {
		t.Errorf("cn.Warehouse = %q", cn.Warehouse)
	}
	if got, want := cn.Ports["starlet"], 9070; got != want {
		t.Errorf("cn.Ports[starlet] = %d, want %d", got, want)
	}
	if cn.CPUCores == nil || *cn.CPUCores != 16 {
		t.Errorf("cn.CPUCores = %v, want 16", cn.CPUCores)
	}
	if topology.ComputeNodes[1].Status != model.StatusDown {
		t.Errorf("dead CN status = %q, want DOWN", topology.ComputeNodes[1].Status)
	}

	summary := topology.Summary
	if summary.ComputeTotal != 2 || summary.ComputeAlive != 1 {
		t.Errorf("compute counts = %d/%d, want 1/2 alive", summary.ComputeAlive, summary.ComputeTotal)
	}
	if summary.TabletTotal != 256 {
		t.Errorf("TabletTotal = %d, want 256 (CN cached tablets count)", summary.TabletTotal)
	}
	// One CN is down, so not healthy — but NOT because "no backends".
	if summary.Healthy {
		t.Error("cluster with a dead CN must not be healthy")
	}
}

func TestTopologySharedDataAllAliveIsHealthy(t *testing.T) {
	repo := sharedDataRepo()
	// Drop the dead CN: leader + one alive CN + zero BEs must be healthy.
	repo.computeNodes = repo.computeNodes[:1]

	topology, err := NewClusterService(repo).Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology() error = %v", err)
	}
	if !topology.Summary.Healthy {
		t.Errorf("shared-data cluster with zero BEs must be healthy via CNs: %+v", topology.Summary)
	}
}

func TestTopologyRunModeResolution(t *testing.T) {
	base := fakeClusterRepo{
		frontends: []repository.Row{{"name": "fe1", "ip": "10.0.0.1", "role": "LEADER", "alive": "true"}},
		backends:  []repository.Row{{"backendid": "10001", "ip": "10.0.0.2", "alive": "true"}},
	}

	t.Run("explicit shared_nothing", func(t *testing.T) {
		repo := base
		repo.runMode = []repository.Row{{"key": "run_mode", "value": "shared_nothing"}}
		topology, _ := NewClusterService(repo).Topology(context.Background())
		if topology.RunMode != model.RunModeSharedNothing {
			t.Errorf("RunMode = %q", topology.RunMode)
		}
	})

	t.Run("empty result means pre-3.0, always shared_nothing", func(t *testing.T) {
		topology, _ := NewClusterService(base).Topology(context.Background())
		if topology.RunMode != model.RunModeSharedNothing {
			t.Errorf("RunMode = %q", topology.RunMode)
		}
	})

	t.Run("config read failure is honest unknown", func(t *testing.T) {
		repo := base
		repo.runModeErr = errors.New("Access denied; you need ADMIN privilege")
		topology, _ := NewClusterService(repo).Topology(context.Background())
		if topology.RunMode != model.RunModeUnknown {
			t.Errorf("RunMode = %q, want unknown", topology.RunMode)
		}
	})

	t.Run("SHOW COMPUTE NODES failure yields empty CN list, not an error", func(t *testing.T) {
		repo := base
		repo.computeErr = errors.New("syntax error")
		topology, err := NewClusterService(repo).Topology(context.Background())
		if err != nil {
			t.Fatalf("Topology() error = %v", err)
		}
		if len(topology.ComputeNodes) != 0 || !topology.Summary.Healthy {
			t.Errorf("topology = %+v", topology.Summary)
		}
	})
}
