import { Activity, SquareTerminal, Workflow } from 'lucide-react'
import { Navigate, Route, Routes } from 'react-router-dom'

import { PagePlaceholder } from '@/components/PagePlaceholder'
import { DashboardLayout } from '@/components/layout/DashboardLayout'
import { DEFAULT_ROUTE } from '@/config/navigation'
import { TopologyView } from '@/features/topology/TopologyView'

export function App() {
  return (
    <Routes>
      <Route path="/" element={<DashboardLayout />}>
        <Route index element={<Navigate to={DEFAULT_ROUTE} replace />} />
        <Route path="topology" element={<TopologyView />} />

        {/* Routes are wired ahead of their features so navigation is complete. */}
        <Route
          path="worksheet"
          element={
            <PagePlaceholder
              icon={SquareTerminal}
              title="SQL Worksheet"
              description="A Monaco-based editor for running StarRocks SQL against the connected cluster."
              planned={[
                'Monaco editor with StarRocks SQL syntax and schema-aware completion',
                'Resizable result grid with dynamic columns',
                'Query profile tab: elapsed time, scanned bytes and rows',
              ]}
            />
          }
        />
        <Route
          path="lineage"
          element={
            <PagePlaceholder
              icon={Workflow}
              title="Data Lineage"
              description="Base table to materialized view dependencies rendered as a DAG."
              planned={[
                'React Flow graph built from information_schema.materialized_views',
                'Custom nodes coloured by last refresh state',
                'Last refresh time and staleness per node',
              ]}
            />
          }
        />
        <Route
          path="metrics"
          element={
            <PagePlaceholder
              icon={Activity}
              title="Metrics"
              description="Time-series resource usage per backend node."
              planned={[
                'ECharts multi-series line chart of CPU usage per BE',
                'Memory and disk pressure over time',
                'Query throughput and slow query counts',
              ]}
            />
          }
        />

        <Route path="*" element={<Navigate to={DEFAULT_ROUTE} replace />} />
      </Route>
    </Routes>
  )
}
