import { CircleAlert, CirclePause, Rss, TriangleAlert } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

import { ErrorState } from '@/components/ErrorState'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { AlertsPanel } from '@/features/loads/components/AlertsPanel'
import { JobsTable } from '@/features/loads/components/JobsTable'
import { useRoutineLoads } from '@/hooks/useRoutineLoads'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { RoutineLoadSummary } from '@/types/routineload'

/**
 * Routine load monitoring: streaming ingestion jobs across all databases with
 * state, throughput, error rows and approximate lag, plus the alert history.
 */
export function RoutineLoadView() {
  const { data, error, isPending, isError, isFetching, refetch } = useRoutineLoads()

  if (isPending) return <RoutineLoadSkeleton />

  if (isError && !data) {
    return (
      <ErrorState
        title="Could not load routine load jobs"
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
        <Notice message="Showing the last successful snapshot — the most recent refresh failed." />
      )}
      {data.warnings?.map((warning) => (
        <Notice key={warning} message={warning} />
      ))}

      <SummaryRow summary={data.summary} />

      <div className="grid items-start gap-6 xl:grid-cols-3">
        <Card size="sm" className="xl:col-span-2">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Rss className="size-4 text-muted-foreground" />
              Jobs
              <span className="font-mono text-xs font-normal text-muted-foreground tabular-nums">
                {data.jobs.length}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {data.jobs.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">
                No routine load jobs found in any database. Create one with{' '}
                <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                  CREATE ROUTINE LOAD
                </code>{' '}
                to stream data from Kafka or Pulsar.
              </p>
            ) : (
              <JobsTable jobs={data.jobs} />
            )}
          </CardContent>
        </Card>

        <AlertsPanel />
      </div>
    </div>
  )
}

function SummaryRow({ summary }: { summary: RoutineLoadSummary }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard
        icon={Rss}
        label="Running"
        value={`${summary.running}/${summary.total}`}
        hint={
          summary.needSchedule > 0
            ? `${summary.needSchedule} waiting for a scheduler slot`
            : 'Jobs actively consuming'
        }
        tone={summary.running === summary.total && summary.total > 0 ? 'good' : 'neutral'}
      />
      <StatCard
        icon={CirclePause}
        label="Paused"
        value={formatNumber(summary.paused)}
        hint={summary.paused > 0 ? 'Resumable — check the reason below' : 'Nothing paused'}
        tone={summary.paused > 0 ? 'bad' : 'good'}
      />
      <StatCard
        icon={CircleAlert}
        label="Cancelled"
        value={formatNumber(summary.cancelled)}
        hint={
          summary.cancelled > 0
            ? 'Terminal — jobs must be recreated'
            : `${summary.stopped} stopped by operators`
        }
        tone={summary.cancelled > 0 ? 'bad' : 'good'}
      />
      <StatCard
        icon={TriangleAlert}
        label="Error rows"
        value={formatNumber(summary.totalErrorRows)}
        hint="Filtered rows across all jobs"
        tone={summary.totalErrorRows > 0 ? 'neutral' : 'good'}
      />
    </div>
  )
}

interface StatCardProps {
  icon: LucideIcon
  label: string
  value: string
  hint: string
  tone: 'neutral' | 'good' | 'bad'
}

function StatCard({ icon: Icon, label, value, hint, tone }: StatCardProps) {
  return (
    <Card size="sm">
      <CardContent className="flex items-start gap-3">
        <span
          className={cn(
            'flex size-8 shrink-0 items-center justify-center rounded-md',
            tone === 'good' && 'bg-success/10 text-success',
            tone === 'bad' && 'bg-destructive/10 text-destructive',
            tone === 'neutral' && 'bg-muted text-muted-foreground',
          )}
        >
          <Icon className="size-4" />
        </span>
        <div className="min-w-0">
          <p className="text-xs tracking-wide text-muted-foreground uppercase">{label}</p>
          <p
            className={cn(
              'truncate font-mono text-lg leading-tight font-semibold tabular-nums',
              tone === 'bad' ? 'text-destructive' : 'text-foreground',
            )}
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

function Notice({ message }: { message: string }) {
  return (
    <div className="flex items-center gap-2 rounded-md bg-warning/10 px-3 py-2 text-xs text-warning ring-1 ring-inset ring-warning/25">
      <TriangleAlert className="size-3.5 shrink-0" />
      {message}
    </div>
  )
}

function RoutineLoadSkeleton() {
  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }, (_, index) => (
          <Skeleton key={index} className="h-20" />
        ))}
      </div>
      <div className="grid gap-6 xl:grid-cols-3">
        <Skeleton className="h-96 xl:col-span-2" />
        <Skeleton className="h-96" />
      </div>
    </div>
  )
}
