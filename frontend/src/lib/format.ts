const NUMBER_FORMAT = new Intl.NumberFormat('en-US')

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const

/** Formats an integer with thousands separators. */
export function formatNumber(value: number | undefined | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  return NUMBER_FORMAT.format(value)
}

/** Formats a byte count as a binary-prefixed size, e.g. 1610612736 → "1.5 GB". */
export function formatBytes(value: number | undefined | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  if (value <= 0) return '0 B'

  const exponent = Math.min(
    Math.floor(Math.log2(value) / 10),
    BYTE_UNITS.length - 1,
  )
  const scaled = value / 1024 ** exponent

  return `${scaled.toFixed(scaled >= 100 || exponent === 0 ? 0 : 1)} ${BYTE_UNITS[exponent]}`
}

/** Formats an already-scaled percentage (1.5 → "1.5%"). */
export function formatPercent(
  value: number | undefined | null,
  fractionDigits = 1,
): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  return `${value.toFixed(fractionDigits)}%`
}

/**
 * Formats an ISO timestamp as a coarse "time ago" label. Used for the freshness
 * indicator, so seconds-level precision is what matters.
 */
export function formatRelativeTime(
  isoTimestamp: string | undefined,
  now: number = Date.now(),
): string {
  if (!isoTimestamp) return '—'

  const parsed = Date.parse(isoTimestamp)
  if (Number.isNaN(parsed)) return isoTimestamp

  const seconds = Math.max(0, Math.round((now - parsed) / 1000))
  if (seconds < 5) return 'just now'
  if (seconds < 60) return `${seconds}s ago`

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`

  return `${Math.floor(hours / 24)}d ago`
}

/**
 * Renders a node's endpoint. StarRocks exposes several ports per node; the
 * query/heartbeat port is the one operators identify a node by.
 */
export function formatEndpoint(
  host: string,
  ports: Record<string, number> | undefined,
  preferred: readonly string[],
): string {
  if (!ports) return host

  for (const key of preferred) {
    const port = ports[key]
    if (port) return `${host}:${port}`
  }
  return host
}
