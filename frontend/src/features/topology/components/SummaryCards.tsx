import { Boxes, Crown, Layers, Server } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent } from '@/components/ui/card'
import { runModeLabel } from '@/features/topology/runMode'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { RunMode, TopologySummary } from '@/types/topology'

type Tone = 'neutral' | 'good' | 'bad'

const TONE_TEXT: Record<Tone, string> = {
  neutral: 'text-foreground',
  good: 'text-success',
  bad: 'text-destructive',
}

const TONE_ICON: Record<Tone, string> = {
  neutral: 'bg-muted text-muted-foreground',
  good: 'bg-success/10 text-success',
  bad: 'bg-destructive/10 text-destructive',
}

export function SummaryCards({
  summary,
  runMode,
}: {
  summary: TopologySummary
  runMode: RunMode
}) {
  const { t } = useTranslation()

  const feDown = summary.frontendTotal - summary.frontendAlive
  const beDown = summary.backendTotal - summary.backendAlive
  const cnDown = summary.computeTotal - summary.computeAlive
  const totalDown = feDown + beDown + cnDown

  const aliveParts = [t('topology.summary.feCount', { count: summary.frontendTotal })]
  if (summary.backendTotal > 0 || runMode !== 'shared_data') {
    aliveParts.push(t('topology.summary.beCount', { count: summary.backendTotal }))
  }
  if (summary.computeTotal > 0 || runMode === 'shared_data') {
    aliveParts.push(t('topology.summary.cnCount', { count: summary.computeTotal }))
  }

  const downParts = [
    feDown > 0 && t('topology.summary.feCount', { count: feDown }),
    beDown > 0 && t('topology.summary.beCount', { count: beDown }),
    cnDown > 0 && t('topology.summary.cnCount', { count: cnDown }),
  ].filter(Boolean)

  const mode = runModeLabel(t, runMode)

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard
        icon={Layers}
        label={t('topology.summary.cluster')}
        value={summary.healthy ? t('topology.summary.healthy') : t('topology.summary.degraded')}
        hint={
          summary.healthy
            ? t('topology.summary.healthyHint', { mode })
            : t('topology.summary.degradedHint', { mode })
        }
        tone={summary.healthy ? 'good' : 'bad'}
      />
      <StatCard
        icon={Crown}
        label={t('topology.summary.feLeader')}
        value={summary.leaderHost || t('topology.summary.none')}
        hint={
          summary.leaderHost
            ? t('topology.summary.quorumHint', {
                alive: summary.electableAlive,
                total: summary.electableTotal,
              })
            : t('topology.summary.noLeaderHint')
        }
        tone={summary.leaderHost && summary.quorumHealthy ? 'neutral' : 'bad'}
        mono
      />
      <StatCard
        icon={Server}
        label={t('topology.summary.nodesAlive')}
        value={`${summary.frontendAlive + summary.backendAlive + summary.computeAlive}/${summary.frontendTotal + summary.backendTotal + summary.computeTotal}`}
        hint={
          totalDown === 0
            ? aliveParts.join(' · ')
            : t('topology.summary.downSuffix', { list: downParts.join(', ') })
        }
        tone={totalDown === 0 ? 'good' : 'bad'}
        mono
      />
      <StatCard
        icon={Boxes}
        label={t('topology.summary.tablets')}
        value={formatNumber(summary.tabletTotal)}
        hint={
          runMode === 'shared_data'
            ? t('topology.summary.tabletsHintSharedData')
            : t('topology.summary.tabletsHint')
        }
        tone="neutral"
        mono
      />
    </div>
  )
}

interface StatCardProps {
  icon: LucideIcon
  label: string
  value: string
  hint: string
  tone: Tone
  mono?: boolean
}

function StatCard({ icon: Icon, label, value, hint, tone, mono }: StatCardProps) {
  return (
    <Card size="sm">
      <CardContent className="flex items-start gap-3">
        <span
          className={cn(
            'flex size-8 shrink-0 items-center justify-center rounded-md',
            TONE_ICON[tone],
          )}
        >
          <Icon className="size-4" />
        </span>
        <div className="min-w-0">
          <p className="text-xs tracking-wide text-muted-foreground uppercase">
            {label}
          </p>
          <p
            className={cn(
              'truncate text-lg leading-tight font-semibold',
              mono && 'font-mono tabular-nums',
              TONE_TEXT[tone],
            )}
            title={value}
          >
            {value}
          </p>
          <p className="truncate text-xs text-muted-foreground" title={hint}>
            {hint}
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
