import { Boxes, Crown, Layers, Server } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

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
  const feDown = summary.frontendTotal - summary.frontendAlive
  const beDown = summary.backendTotal - summary.backendAlive
  const cnDown = summary.computeTotal - summary.computeAlive
  const totalDown = feDown + beDown + cnDown

  const aliveParts = [`${summary.frontendTotal} FE`]
  if (summary.backendTotal > 0 || runMode !== 'shared_data') {
    aliveParts.push(`${summary.backendTotal} BE`)
  }
  if (summary.computeTotal > 0 || runMode === 'shared_data') {
    aliveParts.push(`${summary.computeTotal} CN`)
  }

  const downParts = [
    feDown > 0 && `${feDown} FE`,
    beDown > 0 && `${beDown} BE`,
    cnDown > 0 && `${cnDown} CN`,
  ].filter(Boolean)

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard
        icon={Layers}
        label="Cluster"
        value={summary.healthy ? 'Healthy' : 'Degraded'}
        hint={
          summary.healthy
            ? `${runModeLabel(runMode)} · all nodes alive with a leader`
            : `${runModeLabel(runMode)} · one or more nodes need attention`
        }
        tone={summary.healthy ? 'good' : 'bad'}
      />
      <StatCard
        icon={Crown}
        label="FE leader"
        value={summary.leaderHost || 'None'}
        hint={
          summary.leaderHost
            ? 'Serving metadata writes'
            : 'No frontend has been elected — metadata writes are blocked'
        }
        tone={summary.leaderHost ? 'neutral' : 'bad'}
        mono
      />
      <StatCard
        icon={Server}
        label="Nodes alive"
        value={`${summary.frontendAlive + summary.backendAlive + summary.computeAlive}/${summary.frontendTotal + summary.backendTotal + summary.computeTotal}`}
        hint={totalDown === 0 ? aliveParts.join(' · ') : `${downParts.join(', ')} down`}
        tone={totalDown === 0 ? 'good' : 'bad'}
        mono
      />
      <StatCard
        icon={Boxes}
        label="Tablets"
        value={formatNumber(summary.tabletTotal)}
        hint={
          runMode === 'shared_data'
            ? 'Cached across compute nodes'
            : 'Total across all backends'
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
