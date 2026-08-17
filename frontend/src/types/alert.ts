/**
 * Mirrors the JSON contract of GET /api/v1/alerts and POST /api/v1/alerts/test
 * (see backend/internal/alert/alert.go).
 */

export type AlertSeverity = 'info' | 'warning' | 'critical'

export interface Alert {
  /** Dedup key of the ongoing condition (rule + subject). */
  key: string
  ruleId: string
  severity: AlertSeverity
  title: string
  message: string
  labels?: Record<string, string>
  /** RFC 3339 UTC timestamp stamped at dispatch time. */
  firedAt: string
}

export interface AlertListResponse {
  alerts: Alert[]
}

export interface AlertTestResponse {
  alert: Alert
  /** Per-notifier outcome: "ok" or the delivery error string. */
  results: Record<string, string>
}
