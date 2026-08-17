import { Boxes, Cpu, Crown, Server } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { NodeStatusBadge } from '@/features/topology/components/NodeStatusBadge'
import { UsageBar } from '@/features/topology/components/UsageBar'
import { formatBytes, formatEndpoint, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { ClusterNode } from '@/types/topology'

/** Ports that identify a node, most meaningful first. */
const FE_PORT_PRIORITY = ['query', 'http'] as const
const BE_PORT_PRIORITY = ['heartbeat', 'be', 'http'] as const

export function NodeCard({ node }: { node: ClusterNode }) {
  const isFrontend = node.type === 'FE'
  const isLeader = node.role === 'LEADER'
  const endpoint = formatEndpoint(
    node.host,
    node.ports,
    isFrontend ? FE_PORT_PRIORITY : BE_PORT_PRIORITY,
  )

  return (
    <Card
      size="sm"
      className={cn(
        'transition-shadow hover:shadow-sm',
        // A dead node is the reason someone opened this page — make it obvious.
        node.status === 'DOWN' && 'ring-destructive/30 bg-destructive/[0.02]',
      )}
    >
      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <span
              className={cn(
                'flex size-7 shrink-0 items-center justify-center rounded-md',
                isLeader
                  ? 'bg-primary/10 text-primary'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              {isLeader ? (
                <Crown className="size-4" />
              ) : (
                <Server className="size-4" />
              )}
            </span>
            <div className="min-w-0">
              <CardTitle className="truncate text-sm">{node.name}</CardTitle>
              <p className="truncate font-mono text-xs text-muted-foreground">
                {endpoint}
              </p>
            </div>
          </div>
          <NodeStatusBadge status={node.status} />
        </div>
      </CardHeader>

      <CardContent className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
          {isFrontend && (
            <span
              className={cn(
                'rounded-sm px-1.5 py-0.5 font-medium tracking-wide',
                isLeader
                  ? 'bg-primary/10 text-primary'
                  : 'bg-secondary text-secondary-foreground',
              )}
            >
              {node.role}
            </span>
          )}
          {node.type === 'CN' && (
            <span className="rounded-sm bg-secondary px-1.5 py-0.5 font-medium tracking-wide text-secondary-foreground">
              CN
            </span>
          )}
          {node.warehouse && (
            <WarehouseTag warehouse={node.warehouse} />
          )}
          {node.version && (
            <span className="truncate text-muted-foreground">{node.version}</span>
          )}
        </div>

        {isFrontend ? (
          <FrontendFacts node={node} />
        ) : (
          <BackendFacts node={node} />
        )}

        {node.errMsg && (
          <p className="rounded-md bg-destructive/10 px-2 py-1.5 font-mono text-xs break-words text-destructive">
            {node.errMsg}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

function WarehouseTag({ warehouse }: { warehouse: string }) {
  const { t } = useTranslation()
  return (
    <span className="truncate text-muted-foreground" title={t('topology.node.warehouse')}>
      {warehouse}
    </span>
  )
}

function FrontendFacts({ node }: { node: ClusterNode }) {
  const { t } = useTranslation()
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
      <Fact label={t('topology.node.started')} value={node.startTime} />
      <Fact label={t('topology.node.heartbeat')} value={node.lastHeartbeat} />
    </dl>
  )
}

function BackendFacts({ node }: { node: ClusterNode }) {
  const { t } = useTranslation()
  const hasCapacity = node.totalBytes !== undefined && node.totalBytes > 0

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-3 gap-2">
        <Metric
          icon={Boxes}
          label={t('topology.node.tablets')}
          value={formatNumber(node.tabletNum)}
        />
        <Metric
          icon={Cpu}
          label={t('topology.node.cores')}
          value={formatNumber(node.cpuCores)}
        />
        <Metric
          icon={Server}
          label={t('topology.node.queries')}
          value={formatNumber(node.runningQueries)}
        />
      </div>

      <div className="flex flex-col gap-2">
        <UsageBar
          label={t('topology.node.disk')}
          percent={node.diskUsedPercent}
          caption={
            hasCapacity
              ? `${formatBytes(node.dataUsedBytes)} / ${formatBytes(node.totalBytes)}`
              : undefined
          }
        />
        <UsageBar label={t('topology.node.memory')} percent={node.memUsedPercent} />
      </div>
    </div>
  )
}

function Fact({ label, value }: { label: string; value: string | undefined }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="truncate font-mono tabular-nums text-foreground">
        {value || '—'}
      </dd>
    </>
  )
}

function Metric({
  icon: Icon,
  label,
  value,
}: {
  icon: LucideIcon
  label: string
  value: string
}) {
  return (
    <div className="rounded-md bg-muted/60 px-2 py-1.5">
      <p className="flex items-center gap-1 text-[11px] text-muted-foreground">
        <Icon className="size-3" />
        {label}
      </p>
      <p className="font-mono text-sm tabular-nums text-foreground">{value}</p>
    </div>
  )
}
