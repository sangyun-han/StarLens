import {
  CircleAlert,
  CircleCheck,
  CircleDashed,
  CircleOff,
  CirclePause,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { RoutineLoadState } from '@/types/routineload'

interface StatePresentation {
  label: string
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
    label: 'Running',
    className: 'bg-success/10 text-success ring-1 ring-inset ring-success/25',
    icon: CircleCheck,
  },
  NEED_SCHEDULE: {
    label: 'Scheduling',
    className: 'bg-primary/10 text-primary ring-1 ring-inset ring-primary/25',
    icon: CircleDashed,
  },
  PAUSED: {
    label: 'Paused',
    className: 'bg-warning/10 text-warning ring-1 ring-inset ring-warning/25',
    icon: CirclePause,
  },
  CANCELLED: {
    label: 'Cancelled',
    className:
      'bg-destructive/10 text-destructive ring-1 ring-inset ring-destructive/25',
    icon: CircleAlert,
  },
  STOPPED: {
    label: 'Stopped',
    className: 'bg-muted text-muted-foreground ring-1 ring-inset ring-border',
    icon: CircleOff,
  },
}

const UNKNOWN_PRESENTATION: StatePresentation = {
  label: 'Unknown',
  className: 'bg-muted text-muted-foreground ring-1 ring-inset ring-border',
  icon: CircleDashed,
}

export function JobStateBadge({
  state,
  className,
}: {
  state: RoutineLoadState
  className?: string
}) {
  const presentation = STATE_PRESENTATION[state] ?? {
    ...UNKNOWN_PRESENTATION,
    // Surface a state this client does not know rather than masking it.
    label: state || 'Unknown',
  }
  const Icon = presentation.icon

  return (
    <Badge className={cn('border-transparent', presentation.className, className)}>
      <Icon className="size-3" />
      {presentation.label}
    </Badge>
  )
}
