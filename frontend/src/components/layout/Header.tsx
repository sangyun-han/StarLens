import { Pause, Play, RefreshCw } from 'lucide-react'
import { useLocation } from 'react-router-dom'

import { ClusterStatusPill } from '@/components/layout/ClusterStatusPill'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { NAV_ITEMS } from '@/config/navigation'
import { useNow } from '@/hooks/useNow'
import { useTopology } from '@/hooks/useTopology'
import { ApiError } from '@/lib/api'
import { formatRelativeTime } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useAppStore } from '@/store/useAppStore'

export function Header() {
  const { pathname } = useLocation()
  const now = useNow(10_000)
  const autoRefresh = useAppStore((state) => state.autoRefresh)
  const toggleAutoRefresh = useAppStore((state) => state.toggleAutoRefresh)
  const { data, error, isPending, isFetching, refetch } = useTopology()

  const active = NAV_ITEMS.find((item) => pathname.startsWith(item.to))
  const unreachable = error instanceof ApiError && error.isClusterUnavailable

  return (
    <header className="sticky top-0 z-10 flex h-14 items-center gap-4 border-b border-border bg-card/95 px-6 backdrop-blur">
      <div className="min-w-0 flex-1">
        <h1 className="truncate font-heading text-sm font-semibold text-foreground">
          {active?.label ?? 'StarLens'}
        </h1>
        <p className="truncate text-xs text-muted-foreground">
          {active?.description ?? 'StarRocks management and observability'}
        </p>
      </div>

      <ClusterStatusPill
        summary={data?.summary}
        isLoading={isPending}
        isUnreachable={unreachable}
      />

      <Separator orientation="vertical" className="h-6" />

      <div className="flex items-center gap-1">
        <span className="hidden text-xs text-muted-foreground sm:inline">
          {data ? `Updated ${formatRelativeTime(data.collectedAt, now)}` : 'No data yet'}
        </span>
        <Button
          variant="ghost"
          size="icon"
          onClick={toggleAutoRefresh}
          aria-label={autoRefresh ? 'Pause auto refresh' : 'Resume auto refresh'}
          title={autoRefresh ? 'Auto refresh on (10s)' : 'Auto refresh paused'}
          className={cn('text-muted-foreground', autoRefresh && 'text-primary')}
        >
          {autoRefresh ? <Pause className="size-4" /> : <Play className="size-4" />}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => void refetch()}
          disabled={isFetching}
          aria-label="Refresh cluster data"
          className="text-muted-foreground"
        >
          <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
        </Button>
      </div>
    </header>
  )
}
