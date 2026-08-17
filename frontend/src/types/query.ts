/**
 * Mirrors the JSON contract of POST /api/v1/query and GET /api/v1/databases
 * (see backend/internal/model/query.go).
 */

export interface QueryRequest {
  sql: string
  /** Scopes the statement (USE <db>) when set. */
  database?: string
  /** Requested row cap; the server clamps values above its maximum. */
  maxRows?: number
}

export interface QueryColumn {
  name: string
  /** Driver-reported database type, e.g. VARCHAR, BIGINT. May be empty. */
  type?: string
}

export interface QueryResult {
  columns: QueryColumn[]
  /** Cell values as strings, null for SQL NULL. */
  rows: (string | null)[][]
  rowCount: number
  /** True when the statement produced more rows than maxRows. */
  truncated: boolean
  /** The effective row cap applied to this execution. */
  maxRows: number
  /** Wall-clock execution time measured by the API server. */
  elapsedMs: number
  /** The cleaned SQL that was actually executed. */
  statement: string
  database?: string
}

export interface DatabasesResponse {
  databases: string[]
}
