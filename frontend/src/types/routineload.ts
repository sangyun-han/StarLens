/**
 * Mirrors the JSON contract of GET /api/v1/loads/routine
 * (see backend/internal/model/routineload.go).
 */

export type RoutineLoadState =
  | 'NEED_SCHEDULE'
  | 'RUNNING'
  | 'PAUSED'
  | 'STOPPED'
  | 'CANCELLED'
  | 'UNKNOWN'
  // Future StarRocks releases may report states this client does not know yet.
  | (string & {})

export interface RoutineLoadStatistics {
  totalRows: number
  loadedRows: number
  errorRows: number
  unselectedRows: number
  receivedBytes: number
  taskExecuteTimeMs: number
  committedTaskNum: number
  abortedTaskNum: number
  loadRowsRate?: number
  receivedBytesRate?: number
}

export interface RoutineLoadJob {
  id: string
  name: string
  database: string
  table: string
  state: RoutineLoadState
  dataSourceType?: string
  currentTaskNum?: number

  createTime?: string
  pauseTime?: string
  endTime?: string

  /** Why the job left RUNNING — the first thing to read on a paused job. */
  reasonOfStateChanged?: string
  errorLogUrls?: string
  trackingSql?: string
  otherMsg?: string

  /** Raw per-partition offset JSON as StarRocks reports it. */
  progress?: string
  latestSourcePosition?: string
  /** Approximate messages behind the source log end; absent when unknown. */
  offsetLag?: number

  statistics?: RoutineLoadStatistics
}

export interface RoutineLoadSummary {
  total: number
  running: number
  needSchedule: number
  paused: number
  stopped: number
  cancelled: number
  unhealthy: number
  totalErrorRows: number
}

export interface RoutineLoadSnapshot {
  collectedAt: string
  /** Which read path produced the data. */
  source: 'information_schema' | 'show_routine_load' | (string & {})
  warnings?: string[]
  summary: RoutineLoadSummary
  jobs: RoutineLoadJob[]
}
