import { CircleAlert, CircleCheck, Loader, TriangleAlert } from 'lucide-react'

import { cn } from '@/lib/utils'
import type { TopologySummary } from '@/types/topology'

type Tone = 'healthy' | 'degraded' | 'unknown' | 'loading'

const TONE_STYLES: Record<Tone, string> = {
  healthy: 'bg-success/10 text-success ring-success/25',
  degraded: 'bg-destructive/10 text-destructive ring-destructive/25',
  unknown: 'bg-warning/10 text-warning ring-warning/25',
  loading: 'bg-muted text-muted-foreground ring-border',
}

const TONE_ICONS: Record<Tone, typeof CircleCheck> = {
  healthy: CircleCheck,
  degraded: CircleAlert,
  unknown: TriangleAlert,
  loading: Loader,
}

interface ClusterStatusPillProps {
  summary?: TopologySummary
  isLoading?: boolean
  isUnreachable?: boolean
  className?: string
}

/**
 * The cluster's one-line verdict, shown in the header so the answer to "is
 * anything wrong?" is always on screen regardless of the active page.
 */
export function ClusterStatusPill({
  summary,
  isLoading = false,
  isUnreachable = false,
  className,
}: ClusterStatusPillProps) {
  const { tone, label } = resolveTone({ summary, isLoading, isUnreachable })
  const Icon = TONE_ICONS[tone]

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset',
        TONE_STYLES[tone],
        className,
      )}
    >
      <Icon className={cn('size-3.5', tone === 'loading' && 'animate-spin')} />
      {label}
    </span>
  )
}

function resolveTone({
  summary,
  isLoading,
  isUnreachable,
}: Pick<ClusterStatusPillProps, 'summary' | 'isLoading' | 'isUnreachable'>): {
  tone: Tone
  label: string
} {
  if (isUnreachable) return { tone: 'degraded', label: 'Cluster unreachable' }
  if (!summary) {
    return isLoading
      ? { tone: 'loading', label: 'Checking cluster' }
      : { tone: 'unknown', label: 'Status unknown' }
  }
  if (summary.healthy) return { tone: 'healthy', label: 'Cluster healthy' }

  if (!summary.leaderHost) return { tone: 'degraded', label: 'No FE leader' }

  const down =
    summary.frontendTotal -
    summary.frontendAlive +
    (summary.backendTotal - summary.backendAlive)

  if (down > 0) {
    return {
      tone: 'degraded',
      label: `${down} node${down === 1 ? '' : 's'} down`,
    }
  }
  return { tone: 'unknown', label: 'No backends registered' }
}
