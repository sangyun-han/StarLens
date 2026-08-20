/**
 * Mirrors the JSON contract of the /api/v1/storage endpoints
 * (see backend/internal/model/storage.go).
 */

export interface DatabaseStatistic {
  database: string
  tableNum: number
  partitionNum: number
  tabletNum: number
  replicaNum: number
  /** Tablets missing a healthy replica set — brief during rebalance, alarming when it persists. */
  unhealthyTabletNum: number
  /** Replicas that disagree on their contents. */
  inconsistentTabletNum: number
  cloningTabletNum: number
  errorStateTabletNum: number
}

export interface StorageStatistic {
  collectedAt: string
  databases: DatabaseStatistic[]
  totals: DatabaseStatistic
}

export interface TableSummary {
  database: string
  name: string
  /** Numeric catalog id, used to resolve tablets. */
  id?: string
  /** StarRocks table model: DUP_KEYS, PRIMARY_KEYS, AGG_KEYS, … */
  model?: string
  engine?: string
  partitionKey?: string
  distributeKey?: string
  distributeType?: string
  distributeBucket?: number
  rows?: number
  dataBytes?: number
}

export interface TableList {
  collectedAt: string
  database: string
  tables: TableSummary[]
}

export interface Partition {
  name: string
  rows?: number
  dataBytes?: number
  buckets?: number
  replicationNum?: number
  visibleVersion?: number
  storageMedium?: string
  /** Worst compaction score across the partition's tablets. */
  maxCompactionScore?: number
  avgCompactionScore?: number
  /** False when the partition's tablets sit unevenly across backends. */
  balanced?: boolean
}

export interface BackendTabletLoad {
  backendId: string
  tabletNum: number
  rows: number
  dataBytes: number
  rowsetNum: number
  segmentNum: number
}

/** One max/median comparison; ratios are absent when the median is zero. */
export interface SkewMeasure {
  rowsRatio?: number
  bytesRatio?: number
  skewed: boolean
}

export interface TabletSkew {
  /** Imbalance between backends — what rebalancing fixes. */
  acrossBackends: SkewMeasure
  /** Imbalance between buckets — only a better distribution key fixes this. */
  acrossTablets: SkewMeasure
  skewed: boolean
}

export interface TableDetail {
  collectedAt: string
  table: TableSummary
  partitions: Partition[]
  tabletTotal: number
  rowsetTotal: number
  segmentTotal: number
  /** Compaction pressure: many rowsets on one tablet means reads merge more files. */
  maxRowsetsPerTablet: number
  maxSegmentsPerTablet: number
  /** Tablet replicas not in a healthy steady state. */
  abnormalTablets: number
  backends: BackendTabletLoad[]
  skew: TabletSkew
  warnings?: string[]
}
