import { Pause, Play, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router-dom'

import { ClusterStatusPill } from '@/components/layout/ClusterStatusPill'
import { LanguageSwitcher } from '@/components/layout/LanguageSwitcher'
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
  const { t } = useTranslation()
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
          {active ? t(active.labelKey) : t('common.appName')}
        </h1>
        <p className="truncate text-xs text-muted-foreground">
          {active ? t(active.descriptionKey) : t('common.appDescription')}
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
          {data
            ? t('common.updated', { time: formatRelativeTime(data.collectedAt, now) })
            : t('common.noDataYet')}
        </span>
        <Button
          variant="ghost"
          size="icon"
          onClick={toggleAutoRefresh}
          aria-label={autoRefresh ? t('common.pauseAutoRefresh') : t('common.resumeAutoRefresh')}
          title={autoRefresh ? t('common.autoRefreshOn') : t('common.autoRefreshPaused')}
          className={cn('text-muted-foreground', autoRefresh && 'text-primary')}
        >
          {autoRefresh ? <Pause className="size-4" /> : <Play className="size-4" />}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => void refetch()}
          disabled={isFetching}
          aria-label={t('common.refresh')}
          className="text-muted-foreground"
        >
          <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
        </Button>
        <LanguageSwitcher />
      </div>
    </header>
  )
}
