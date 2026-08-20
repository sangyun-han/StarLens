import { Activity, Workflow } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Suspense, lazy } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, Route, Routes } from 'react-router-dom'

import { PagePlaceholder } from '@/components/PagePlaceholder'
import { DashboardLayout } from '@/components/layout/DashboardLayout'
import { Skeleton } from '@/components/ui/skeleton'
import { DEFAULT_ROUTE } from '@/config/navigation'
import { RoutineLoadView } from '@/features/loads/RoutineLoadView'
import { StorageView } from '@/features/storage/StorageView'
import { TopologyView } from '@/features/topology/TopologyView'

// Monaco is heavy; the worksheet (and the editor with it) loads on demand so
// the monitoring pages stay in a lean main bundle.
const SqlWorksheet = lazy(() => import('@/features/worksheet/SqlWorksheet'))

export function App() {
  return (
    <Routes>
      <Route path="/" element={<DashboardLayout />}>
        <Route index element={<Navigate to={DEFAULT_ROUTE} replace />} />
        <Route path="topology" element={<TopologyView />} />
        <Route path="loads" element={<RoutineLoadView />} />
        <Route path="storage" element={<StorageView />} />

        <Route
          path="worksheet"
          element={
            <Suspense fallback={<Skeleton className="h-[calc(100vh-8.5rem)]" />}>
              <SqlWorksheet />
            </Suspense>
          }
        />

        {/* Routes are wired ahead of their features so navigation is complete. */}
        <Route
          path="lineage"
          element={<PlaceholderRoute page="lineage" icon={Workflow} />}
        />
        <Route
          path="metrics"
          element={<PlaceholderRoute page="metrics" icon={Activity} />}
        />

        <Route path="*" element={<Navigate to={DEFAULT_ROUTE} replace />} />
      </Route>
    </Routes>
  )
}

/** Localized placeholder content for routes whose feature is not built yet. */
function PlaceholderRoute({
  page,
  icon,
}: {
  page: 'lineage' | 'metrics'
  icon: LucideIcon
}) {
  const { t } = useTranslation()
  const planned = t(`placeholders.${page}.planned`, {
    returnObjects: true,
  }) as string[]

  return (
    <PagePlaceholder
      icon={icon}
      title={t(`placeholders.${page}.title`)}
      description={t(`placeholders.${page}.description`)}
      planned={planned}
    />
  )
}
