// Package model holds the API-facing domain types shared by the service and
// API layers. Field tags define the JSON contract consumed by the frontend.
package model

import "time"

// NodeType distinguishes StarRocks node classes.
type NodeType string

const (
	NodeTypeFrontend NodeType = "FE"
	NodeTypeBackend  NodeType = "BE"
	// NodeTypeCompute is a stateless CN. The compute layer of a shared-data
	// cluster, and optional query-scaling nodes in a shared-nothing one.
	NodeTypeCompute NodeType = "CN"
)

// Deployment modes as reported by the FE config item `run_mode` (v3.0+).
const (
	// RunModeSharedNothing: classic coupled storage/compute, data on BE disks.
	RunModeSharedNothing = "shared_nothing"
	// RunModeSharedData: storage/compute separation, data on object storage,
	// stateless CNs as the compute layer.
	RunModeSharedData = "shared_data"
	// RunModeUnknown: the config could not be read (e.g. missing ADMIN
	// privilege), so the mode is undetermined.
	RunModeUnknown = "unknown"
)

// Node roles. Frontends elect one LEADER; FOLLOWERs are electable replicas and
// OBSERVERs are read-only metadata replicas.
const (
	RoleLeader   = "LEADER"
	RoleFollower = "FOLLOWER"
	RoleObserver = "OBSERVER"
	RoleBackend  = "BACKEND"
	RoleCompute  = "COMPUTE"
	RoleUnknown  = "UNKNOWN"
)

// NodeStatus is the traffic-light state rendered by the topology view.
type NodeStatus string

const (
	// StatusHealthy means the node answered its last heartbeat.
	StatusHealthy NodeStatus = "HEALTHY"
	// StatusDown means StarRocks considers the node dead (Alive = false).
	StatusDown NodeStatus = "DOWN"
	// StatusDecommissioned means the node is alive but draining its tablets.
	StatusDecommissioned NodeStatus = "DECOMMISSIONED"
)

// Node is one frontend or backend in the cluster.
type Node struct {
	// ID is stable across polls: "fe:<name>" or "be:<backendId>".
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Type   NodeType   `json:"type"`
	Role   string     `json:"role"`
	Status NodeStatus `json:"status"`
	Alive  bool       `json:"alive"`

	Host          string         `json:"host"`
	Ports         map[string]int `json:"ports,omitempty"`
	Version       string         `json:"version,omitempty"`
	StartTime     string         `json:"startTime,omitempty"`
	LastHeartbeat string         `json:"lastHeartbeat,omitempty"`
	ErrMsg        string         `json:"errMsg,omitempty"`
	// Warehouse is the multi-warehouse assignment of a CN, when reported.
	Warehouse string `json:"warehouse,omitempty"`

	// Frontend-only HA and metadata-replication state.
	//
	// ReplayedJournalID is how far this FE has applied the metadata edit log;
	// JournalLag is how far it trails the leader (0 on the leader itself). A
	// follower that stops replaying cannot take over, so lag is how a degraded
	// HA setup shows itself while every node still reports Alive.
	ReplayedJournalID *int64 `json:"replayedJournalId,omitempty"`
	JournalLag        *int64 `json:"journalLag,omitempty"`
	// IsHelper marks a frontend that participates in the election quorum.
	IsHelper *bool `json:"isHelper,omitempty"`
	// Joined reports whether the frontend finished joining the cluster.
	Joined *bool `json:"joined,omitempty"`
	// ClusterID must match across frontends; a disagreement means a node
	// belongs to a different cluster.
	ClusterID string `json:"clusterId,omitempty"`

	// Backend-only capacity and workload metrics. Pointers stay nil when the
	// running StarRocks version does not report the column.
	TabletNum       *int64   `json:"tabletNum,omitempty"`
	CPUCores        *int64   `json:"cpuCores,omitempty"`
	RunningQueries  *int64   `json:"runningQueries,omitempty"`
	DiskUsedPercent *float64 `json:"diskUsedPercent,omitempty"`
	MemUsedPercent  *float64 `json:"memUsedPercent,omitempty"`
	CPUUsedPercent  *float64 `json:"cpuUsedPercent,omitempty"`
	DataUsedBytes   *int64   `json:"dataUsedBytes,omitempty"`
	TotalBytes      *int64   `json:"totalBytes,omitempty"`
	AvailableBytes  *int64   `json:"availableBytes,omitempty"`
}

// TopologySummary is the header strip of the topology dashboard.
type TopologySummary struct {
	FrontendTotal int `json:"frontendTotal"`
	FrontendAlive int `json:"frontendAlive"`
	BackendTotal  int `json:"backendTotal"`
	BackendAlive  int `json:"backendAlive"`
	ComputeTotal  int `json:"computeTotal"`
	ComputeAlive  int `json:"computeAlive"`

	// LeaderHost is the FE currently serving metadata writes, empty if no
	// leader was elected — which is itself an alarming state.
	LeaderHost string `json:"leaderHost"`
	// ElectableAlive/ElectableTotal count LEADER+FOLLOWER frontends — the pool
	// that forms the metadata quorum. OBSERVERs never vote, so they are
	// excluded even though they replicate metadata.
	ElectableAlive int `json:"electableAlive"`
	ElectableTotal int `json:"electableTotal"`
	// QuorumHealthy is true while a majority of electable frontends is alive.
	// Losing quorum blocks metadata writes even if queries still serve.
	QuorumHealthy bool `json:"quorumHealthy"`
	// MaxJournalLag is the largest metadata replication lag among live
	// non-leader frontends; nil when no frontend reports a journal position.
	MaxJournalLag *int64 `json:"maxJournalLag,omitempty"`
	// ClusterIDMismatch is true when frontends disagree on ClusterId.
	ClusterIDMismatch bool `json:"clusterIdMismatch"`
	// TabletTotal is the sum of tablets across live backends and compute nodes.
	TabletTotal int64 `json:"tabletTotal"`
	// Healthy is true only when a leader exists, every node is alive, and the
	// cluster has at least one compute-capable node (BE or CN).
	Healthy bool `json:"healthy"`
}

// Topology is the payload of GET /api/v1/cluster/topology.
type Topology struct {
	CollectedAt time.Time `json:"collectedAt"`
	// RunMode is one of the RunMode* constants: how this cluster is deployed.
	RunMode   string          `json:"runMode"`
	Summary   TopologySummary `json:"summary"`
	Frontends []Node          `json:"frontends"`
	Backends  []Node          `json:"backends"`
	// ComputeNodes is the CN list. Empty on shared-nothing clusters without
	// added CNs, and on StarRocks versions predating compute nodes.
	ComputeNodes []Node `json:"computeNodes"`
}
