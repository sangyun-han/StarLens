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

export type WebhookFormat = 'generic' | 'slack'

/** Read shape of GET/PUT /api/v1/alerts/config. The webhook URL never leaves
 * the server — only a configured flag plus a masked hint. */
export interface AlertConfigView {
  /** False when ALERT_CONFIG_UI=false: config is visible but read-only. */
  editable: boolean
  /** JSON field names currently overridden via the UI (vs. environment). */
  overridden: string[]
  config: {
    enabled: boolean
    /** Go duration strings, e.g. "30s", "10m". */
    pollInterval: string
    cooldown: string
    webhookConfigured: boolean
    /** Masked destination, e.g. "https://hooks.slack.com/•••". */
    webhookHint?: string
    webhookFormat: WebhookFormat
    /** Fraction in [0, 1]; the UI shows percent. */
    errorRowsRatio: number
    errorRowsMinTotal: number
    maxOffsetLag: number
    maxJournalLag: number
  }
}

/** Write shape of PUT /api/v1/alerts/config. Absent fields stay unchanged;
 * reset=true first reverts every override to the environment defaults. */
export interface AlertConfigPatch {
  reset?: boolean
  enabled?: boolean
  pollInterval?: string
  cooldown?: string
  /** Omit = unchanged; "" = explicitly no webhook (overrides env). */
  webhookUrl?: string
  webhookFormat?: WebhookFormat
  errorRowsRatio?: number
  errorRowsMinTotal?: number
  maxOffsetLag?: number
  maxJournalLag?: number
}
