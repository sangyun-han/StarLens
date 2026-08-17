import { Activity, SquareTerminal, Workflow } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Navigate, Route, Routes } from 'react-router-dom'

import { PagePlaceholder } from '@/components/PagePlaceholder'
import { DashboardLayout } from '@/components/layout/DashboardLayout'
import { DEFAULT_ROUTE } from '@/config/navigation'
import { RoutineLoadView } from '@/features/loads/RoutineLoadView'
import { TopologyView } from '@/features/topology/TopologyView'

export function App() {
  return (
    <Routes>
      <Route path="/" element={<DashboardLayout />}>
        <Route index element={<Navigate to={DEFAULT_ROUTE} replace />} />
        <Route path="topology" element={<TopologyView />} />
        <Route path="loads" element={<RoutineLoadView />} />

        {/* Routes are wired ahead of their features so navigation is complete. */}
        <Route
          path="worksheet"
          element={<PlaceholderRoute page="worksheet" icon={SquareTerminal} />}
        />
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
  page: 'worksheet' | 'lineage' | 'metrics'
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
