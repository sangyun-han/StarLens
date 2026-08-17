import { Cpu, HardDrive, Server, TriangleAlert } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

import { ErrorState } from '@/components/ErrorState'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { NodeCard } from '@/features/topology/components/NodeCard'
import { SummaryCards } from '@/features/topology/components/SummaryCards'
import { useTopology } from '@/hooks/useTopology'
import type { ClusterNode } from '@/types/topology'

/**
 * Cluster topology dashboard: cluster-level summary on top, then one card per
 * frontend and backend with liveness, role and capacity.
 */
export function TopologyView() {
  const { data, error, isPending, isError, isFetching, refetch } = useTopology()

  if (isPending) return <TopologySkeleton />

  // With placeholderData the last good snapshot survives a failed poll, so an
  // error only takes over the page when there is nothing to show.
  if (isError && !data) {
    return (
      <ErrorState
        title="Could not load cluster topology"
        error={error}
        onRetry={() => void refetch()}
        isRetrying={isFetching}
      />
    )
  }

  if (!data) return null

  return (
    <div className="flex flex-col gap-6">
      {isError && (
        <StaleNotice
          message="Showing the last successful snapshot — the most recent refresh failed."
        />
      )}

      <SummaryCards summary={data.summary} runMode={data.runMode} />

      <NodeSection
        icon={Server}
        title="Frontends"
        subtitle="Metadata, SQL parsing and query planning"
        nodes={data.frontends}
        emptyMessage="No frontends reported. SHOW FRONTENDS returned no rows."
      />

      {/* In shared-data mode the BE section is expected to be empty — hide it
          rather than showing a scary empty state, and vice versa for CNs. */}
      {(data.backends.length > 0 || data.runMode !== 'shared_data') && (
        <NodeSection
          icon={HardDrive}
          title="Backends"
          subtitle="Coupled storage and query execution"
          nodes={data.backends}
          emptyMessage={
            data.runMode === 'shared_nothing'
              ? 'No backends registered. The cluster cannot serve queries until a BE joins.'
              : 'No backends registered.'
          }
        />
      )}

      {(data.computeNodes.length > 0 || data.runMode === 'shared_data') && (
        <NodeSection
          icon={Cpu}
          title="Compute Nodes"
          subtitle="Stateless query execution over shared storage"
          nodes={data.computeNodes}
          emptyMessage="No compute nodes registered. A shared-data cluster cannot serve queries until a CN joins."
        />
      )}
    </div>
  )
}

interface NodeSectionProps {
  icon: LucideIcon
  title: string
  subtitle: string
  nodes: ClusterNode[]
  emptyMessage: string
}

function NodeSection({
  icon: Icon,
  title,
  subtitle,
  nodes,
  emptyMessage,
}: NodeSectionProps) {
  const down = nodes.filter((node) => !node.alive).length

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <Icon className="size-4 text-muted-foreground" />
        <h2 className="font-heading text-sm font-semibold text-foreground">
          {title}
        </h2>
        <span className="font-mono text-xs text-muted-foreground tabular-nums">
          {nodes.length - down}/{nodes.length} alive
        </span>
        <span className="hidden text-xs text-muted-foreground md:inline">
          · {subtitle}
        </span>
      </div>

      {nodes.length === 0 ? (
        <Card>
          <CardContent className="text-sm text-muted-foreground">
            {emptyMessage}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {nodes.map((node) => (
            <NodeCard key={node.id} node={node} />
          ))}
        </div>
      )}
    </section>
  )
}

function StaleNotice({ message }: { message: string }) {
  return (
    <div className="flex items-center gap-2 rounded-md bg-warning/10 px-3 py-2 text-xs text-warning ring-1 ring-inset ring-warning/25">
      <TriangleAlert className="size-3.5 shrink-0" />
      {message}
    </div>
  )
}

function TopologySkeleton() {
  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }, (_, index) => (
          <Skeleton key={index} className="h-20" />
        ))}
      </div>
      {['frontends', 'backends'].map((section) => (
        <div key={section} className="flex flex-col gap-3">
          <Skeleton className="h-4 w-40" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 3 }, (_, index) => (
              <Skeleton key={index} className="h-40" />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
