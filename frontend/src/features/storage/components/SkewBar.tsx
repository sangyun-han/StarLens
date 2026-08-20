import { useTranslation } from 'react-i18next'

import { formatBytes, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { BackendTabletLoad } from '@/types/storage'

/**
 * Per-backend data distribution. Bars are drawn against the largest backend so
 * an uneven spread is visible at a glance rather than inferred from numbers.
 */
export function BackendDistribution({ backends }: { backends: BackendTabletLoad[] }) {
  const { t } = useTranslation()
  const maxRows = Math.max(...backends.map((b) => b.rows), 1)

  return (
    <div className="flex flex-col gap-2">
      {backends.map((backend) => (
        <div key={backend.backendId} className="flex flex-col gap-1">
          <div className="flex items-baseline justify-between gap-2 text-xs">
            <span className="font-mono text-foreground">BE {backend.backendId}</span>
            <span className="font-mono text-muted-foreground tabular-nums">
              {t('storage.detail.backendSummary', {
                tablets: formatNumber(backend.tabletNum),
                rows: formatNumber(backend.rows),
                size: formatBytes(backend.dataBytes),
              })}
            </span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-primary transition-[width] duration-300"
              style={{ width: `${(backend.rows / maxRows) * 100}%` }}
            />
          </div>
        </div>
      ))}
    </div>
  )
}

/** One skew level: the ratio, plus what it means and what fixes it. */
export function SkewCard({
  title,
  hint,
  rowsRatio,
  bytesRatio,
  skewed,
}: {
  title: string
  hint: string
  rowsRatio?: number
  bytesRatio?: number
  skewed: boolean
}) {
  const { t } = useTranslation()
  const measurable = rowsRatio !== undefined || bytesRatio !== undefined

  return (
    <div
      className={cn(
        'flex flex-col gap-1 rounded-md px-3 py-2 ring-1 ring-inset',
        !measurable && 'bg-muted/50 ring-border',
        measurable && skewed && 'bg-destructive/5 ring-destructive/25',
        measurable && !skewed && 'bg-success/5 ring-success/25',
      )}
    >
      <p className="text-xs tracking-wide text-muted-foreground uppercase">{title}</p>
      <p
        className={cn(
          'font-mono text-lg leading-tight font-semibold tabular-nums',
          measurable && skewed ? 'text-destructive' : 'text-foreground',
        )}
      >
        {rowsRatio === undefined
          ? t('storage.detail.notMeasurable')
          : t('storage.detail.ratio', { value: formatRatio(rowsRatio) })}
      </p>
      {bytesRatio !== undefined && (
        <p className="font-mono text-xs text-muted-foreground tabular-nums">
          {t('storage.detail.bytesRatio', { value: formatRatio(bytesRatio) })}
        </p>
      )}
      <p className="text-xs text-muted-foreground">{hint}</p>
    </div>
  )
}

/** Ratios span orders of magnitude; keep small ones precise, big ones short. */
function formatRatio(value: number): string {
  if (value >= 100) return Math.round(value).toLocaleString('en-US')
  if (value >= 10) return value.toFixed(1)
  return value.toFixed(2)
}
