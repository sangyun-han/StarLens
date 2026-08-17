import { cn } from '@/lib/utils'
import { formatPercent } from '@/lib/format'

/** Disk and memory pressure thresholds at which a backend needs attention. */
const WARNING_THRESHOLD = 70
const CRITICAL_THRESHOLD = 85

interface UsageBarProps {
  label: string
  /** Already-scaled percentage (1.5 means 1.5%). Undefined renders as unknown. */
  percent: number | undefined
  /** Optional right-aligned caption, e.g. "1.5 GB / 100 GB". */
  caption?: string
}

export function UsageBar({ label, percent, caption }: UsageBarProps) {
  const known = percent !== undefined && !Number.isNaN(percent)
  const clamped = known ? Math.min(100, Math.max(0, percent)) : 0

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-mono tabular-nums text-foreground">
          {caption ? `${caption} · ${formatPercent(percent)}` : formatPercent(percent)}
        </span>
      </div>
      <div
        className="h-1.5 overflow-hidden rounded-full bg-muted"
        role="meter"
        aria-label={label}
        aria-valuenow={known ? clamped : undefined}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className={cn(
            'h-full rounded-full transition-[width] duration-300',
            clamped >= CRITICAL_THRESHOLD && 'bg-destructive',
            clamped >= WARNING_THRESHOLD &&
              clamped < CRITICAL_THRESHOLD &&
              'bg-warning',
            clamped < WARNING_THRESHOLD && 'bg-success',
          )}
          // An unknown value leaves the track empty rather than showing a full
          // grey bar, which reads as 100%.
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  )
}
