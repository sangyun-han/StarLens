import { Activity, Network, Rss, SquareTerminal, Workflow } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export interface NavItem {
  to: string
  /** i18n key for the menu label, e.g. "nav.topology.label". */
  labelKey: string
  /** i18n key for the page subtitle shown in the header. */
  descriptionKey: string
  icon: LucideIcon
  /** Placeholder routes render a "planned" state instead of a dead link. */
  available: boolean
}

export const NAV_ITEMS: readonly NavItem[] = [
  {
    to: '/topology',
    labelKey: 'nav.topology.label',
    descriptionKey: 'nav.topology.description',
    icon: Network,
    available: true,
  },
  {
    to: '/loads',
    labelKey: 'nav.loads.label',
    descriptionKey: 'nav.loads.description',
    icon: Rss,
    available: true,
  },
  {
    to: '/worksheet',
    labelKey: 'nav.worksheet.label',
    descriptionKey: 'nav.worksheet.description',
    icon: SquareTerminal,
    available: true,
  },
  {
    to: '/lineage',
    labelKey: 'nav.lineage.label',
    descriptionKey: 'nav.lineage.description',
    icon: Workflow,
    available: false,
  },
  {
    to: '/metrics',
    labelKey: 'nav.metrics.label',
    descriptionKey: 'nav.metrics.description',
    icon: Activity,
    available: false,
  },
]

export const DEFAULT_ROUTE = '/topology'
