import { Activity, Network, SquareTerminal, Workflow } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export interface NavItem {
  to: string
  label: string
  /** Shown as the page subtitle in the header. */
  description: string
  icon: LucideIcon
  /** Placeholder routes render a "planned" state instead of a dead link. */
  available: boolean
}

export const NAV_ITEMS: readonly NavItem[] = [
  {
    to: '/topology',
    label: 'Topology',
    description: 'Frontend and backend nodes, liveness and capacity',
    icon: Network,
    available: true,
  },
  {
    to: '/worksheet',
    label: 'SQL Worksheet',
    description: 'Run StarRocks SQL and inspect the query profile',
    icon: SquareTerminal,
    available: false,
  },
  {
    to: '/lineage',
    label: 'Lineage',
    description: 'Materialized view dependencies as a pipeline graph',
    icon: Workflow,
    available: false,
  },
  {
    to: '/metrics',
    label: 'Metrics',
    description: 'CPU, memory and query throughput over time',
    icon: Activity,
    available: false,
  },
]

export const DEFAULT_ROUTE = '/topology'
