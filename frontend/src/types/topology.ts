/**
 * Mirrors the JSON contract of GET /api/v1/cluster/topology
 * (see backend/internal/model/cluster.go).
 */

export type NodeType = 'FE' | 'BE' | 'CN'

/** How the cluster is deployed, from the FE `run_mode` config (v3.0+). */
export type RunMode = 'shared_nothing' | 'shared_data' | 'unknown' | (string & {})

export type NodeStatus = 'HEALTHY' | 'DOWN' | 'DECOMMISSIONED'

export type NodeRole =
  | 'LEADER'
  | 'FOLLOWER'
  | 'OBSERVER'
  | 'BACKEND'
  | 'COMPUTE'
  | 'UNKNOWN'
  // Future StarRocks releases may report roles this client does not know yet.
  | (string & {})

export interface ClusterNode {
  /** Stable across polls: `fe:<name>` or `be:<backendId>`. */
  id: string
  name: string
  type: NodeType
  role: NodeRole
  status: NodeStatus
  alive: boolean
  host: string
  /** Named ports, e.g. `{ query: 9030, http: 8030 }`. */
  ports?: Record<string, number>
  version?: string
  /** StarRocks-local wall clock strings, not ISO timestamps. */
  startTime?: string
  lastHeartbeat?: string
  errMsg?: string
  /** Multi-warehouse assignment of a CN, when reported. */
  warehouse?: string

  /** BE/CN-only fields; absent when the StarRocks version omits the column. */
  tabletNum?: number
  cpuCores?: number
  runningQueries?: number
  diskUsedPercent?: number
  memUsedPercent?: number
  cpuUsedPercent?: number
  dataUsedBytes?: number
  totalBytes?: number
  availableBytes?: number
}

export interface TopologySummary {
  frontendTotal: number
  frontendAlive: number
  backendTotal: number
  backendAlive: number
  computeTotal: number
  computeAlive: number
  /** Empty when no frontend has been elected leader. */
  leaderHost: string
  tabletTotal: number
  healthy: boolean
}

export interface Topology {
  /** RFC 3339 UTC timestamp stamped by the API. */
  collectedAt: string
  runMode: RunMode
  summary: TopologySummary
  frontends: ClusterNode[]
  backends: ClusterNode[]
  /** CNs — the compute layer of a shared-data cluster. */
  computeNodes: ClusterNode[]
}
