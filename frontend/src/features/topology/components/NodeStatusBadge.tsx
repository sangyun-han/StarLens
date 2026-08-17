import { CircleAlert, CircleCheck, TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { NodeStatus } from '@/types/topology'

interface StatusPresentation {
  labelKey: string
  className: string
  icon: typeof CircleCheck
}

/**
 * Alive nodes read green, dead nodes red, draining nodes amber. Each state
 * carries an icon as well as a hue so status never depends on color alone.
 */
const STATUS_PRESENTATION: Record<NodeStatus, StatusPresentation> = {
  HEALTHY: {
    labelKey: 'nodeStatus.alive',
    className: 'bg-success/10 text-success ring-1 ring-inset ring-success/25',
    icon: CircleCheck,
  },
  DOWN: {
    labelKey: 'nodeStatus.down',
    className:
      'bg-destructive/10 text-destructive ring-1 ring-inset ring-destructive/25',
    icon: CircleAlert,
  },
  DECOMMISSIONED: {
    labelKey: 'nodeStatus.draining',
    className: 'bg-warning/10 text-warning ring-1 ring-inset ring-warning/25',
    icon: TriangleAlert,
  },
}

export function NodeStatusBadge({
  status,
  className,
}: {
  status: NodeStatus
  className?: string
}) {
  const { t } = useTranslation()
  const presentation = STATUS_PRESENTATION[status] ?? STATUS_PRESENTATION.DOWN
  const Icon = presentation.icon

  return (
    <Badge className={cn('border-transparent', presentation.className, className)}>
      <Icon className="size-3" />
      {t(presentation.labelKey)}
    </Badge>
  )
}
