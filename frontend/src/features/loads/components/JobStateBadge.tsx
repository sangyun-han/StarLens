import {
  CircleAlert,
  CircleCheck,
  CircleDashed,
  CircleOff,
  CirclePause,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { RoutineLoadState } from '@/types/routineload'

interface StatePresentation {
  labelKey: string
  className: string
  icon: LucideIcon
}

/**
 * Each state carries an icon as well as a hue so status never depends on color
 * alone. PAUSED is amber (recoverable), CANCELLED is red (needs intervention),
 * STOPPED is neutral (user intent).
 */
const STATE_PRESENTATION: Record<string, StatePresentation> = {
  RUNNING: {
    labelKey: 'jobState.RUNNING',
    className: 'bg-success/10 text-success ring-1 ring-inset ring-success/25',
    icon: CircleCheck,
  },
  NEED_SCHEDULE: {
    labelKey: 'jobState.NEED_SCHEDULE',
    className: 'bg-primary/10 text-primary ring-1 ring-inset ring-primary/25',
    icon: CircleDashed,
  },
  PAUSED: {
    labelKey: 'jobState.PAUSED',
    className: 'bg-warning/10 text-warning ring-1 ring-inset ring-warning/25',
    icon: CirclePause,
  },
  CANCELLED: {
    labelKey: 'jobState.CANCELLED',
    className:
      'bg-destructive/10 text-destructive ring-1 ring-inset ring-destructive/25',
    icon: CircleAlert,
  },
  STOPPED: {
    labelKey: 'jobState.STOPPED',
    className: 'bg-muted text-muted-foreground ring-1 ring-inset ring-border',
    icon: CircleOff,
  },
}

const UNKNOWN_CLASS = 'bg-muted text-muted-foreground ring-1 ring-inset ring-border'

export function JobStateBadge({
  state,
  className,
}: {
  state: RoutineLoadState
  className?: string
}) {
  const { t } = useTranslation()
  const presentation = STATE_PRESENTATION[state]
  const Icon = presentation?.icon ?? CircleDashed

  return (
    <Badge
      className={cn(
        'border-transparent',
        presentation?.className ?? UNKNOWN_CLASS,
        className,
      )}
    >
      <Icon className="size-3" />
      {/* Surface a state this client does not know rather than masking it. */}
      {presentation ? t(presentation.labelKey) : state || t('jobState.UNKNOWN')}
    </Badge>
  )
}
