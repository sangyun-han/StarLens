import { CircleAlert, CircleCheck, Loader, TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

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
  const { t } = useTranslation()
  const { tone, labelKey, count } = resolveTone({ summary, isLoading, isUnreachable })
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
      {t(labelKey, { count })}
    </span>
  )
}

function resolveTone({
  summary,
  isLoading,
  isUnreachable,
}: Pick<ClusterStatusPillProps, 'summary' | 'isLoading' | 'isUnreachable'>): {
  tone: Tone
  labelKey: string
  count?: number
} {
  if (isUnreachable) return { tone: 'degraded', labelKey: 'status.clusterUnreachable' }
  if (!summary) {
    return isLoading
      ? { tone: 'loading', labelKey: 'status.checkingCluster' }
      : { tone: 'unknown', labelKey: 'status.statusUnknown' }
  }
  if (summary.healthy) return { tone: 'healthy', labelKey: 'status.clusterHealthy' }

  if (!summary.leaderHost) return { tone: 'degraded', labelKey: 'status.noFeLeader' }

  const down =
    summary.frontendTotal -
    summary.frontendAlive +
    (summary.backendTotal - summary.backendAlive) +
    (summary.computeTotal - summary.computeAlive)

  if (down > 0) {
    return { tone: 'degraded', labelKey: 'status.nodesDown', count: down }
  }
  return { tone: 'unknown', labelKey: 'status.noBackendsRegistered' }
}
