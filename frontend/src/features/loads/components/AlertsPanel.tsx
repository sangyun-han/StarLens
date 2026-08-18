import { BellRing, CircleAlert, Info, SendHorizontal, TriangleAlert } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AlertSettingsDialog } from '@/features/loads/components/AlertSettingsDialog'
import { useAlerts, useTestAlert } from '@/hooks/useAlerts'
import { useNow } from '@/hooks/useNow'
import { formatRelativeTime } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { Alert, AlertSeverity } from '@/types/alert'

const SEVERITY_ICON: Record<AlertSeverity, LucideIcon> = {
  info: Info,
  warning: TriangleAlert,
  critical: CircleAlert,
}

const SEVERITY_STYLE: Record<AlertSeverity, string> = {
  info: 'bg-primary/10 text-primary',
  warning: 'bg-warning/10 text-warning',
  critical: 'bg-destructive/10 text-destructive',
}

/**
 * The fired-alert history plus a test-fire button, so an operator can verify a
 * webhook channel end to end without waiting for a real incident.
 */
export function AlertsPanel() {
  const { t } = useTranslation()
  const { data } = useAlerts()
  const testAlert = useTestAlert()
  const now = useNow(10_000)
  const [testOutcome, setTestOutcome] = useState<string | null>(null)

  const alerts = data?.alerts ?? []

  const fireTest = () => {
    setTestOutcome(null)
    testAlert.mutate(undefined, {
      onSuccess: (response) => {
        const failed = Object.entries(response.results).filter(
          ([, outcome]) => outcome !== 'ok',
        )
        setTestOutcome(
          failed.length === 0
            ? t('alerts.deliveredAll')
            : t('alerts.deliveryFailed', {
                list: failed.map(([name, err]) => `${name} (${err})`).join(', '),
              }),
        )
      },
      onError: () => setTestOutcome(t('alerts.apiUnreachable')),
    })
  }

  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BellRing className="size-4 text-muted-foreground" />
          {t('alerts.recentAlerts')}
        </CardTitle>
        <CardAction className="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            onClick={fireTest}
            disabled={testAlert.isPending}
          >
            <SendHorizontal className="size-3.5" />
            {t('common.test')}
          </Button>
          <AlertSettingsDialog />
        </CardAction>
      </CardHeader>

      <CardContent className="flex flex-col gap-2">
        {testOutcome && (
          <p className="rounded-md bg-muted px-2.5 py-1.5 text-xs text-muted-foreground">
            {testOutcome}
          </p>
        )}

        {alerts.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            {t('alerts.empty')}
          </p>
        ) : (
          <ul className="flex max-h-160 flex-col gap-1.5 overflow-y-auto">
            {alerts.map((alert, index) => (
              <AlertRow key={`${alert.key}-${alert.firedAt}-${index}`} alert={alert} now={now} />
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

function AlertRow({ alert, now }: { alert: Alert; now: number }) {
  const Icon = SEVERITY_ICON[alert.severity] ?? Info

  return (
    <li className="flex items-start gap-2.5 rounded-md px-2 py-1.5 hover:bg-muted/60">
      <span
        className={cn(
          'mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-md',
          SEVERITY_STYLE[alert.severity] ?? SEVERITY_STYLE.info,
        )}
      >
        <Icon className="size-3.5" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-foreground" title={alert.title}>
          {alert.title}
        </p>
        {alert.message && (
          <p className="line-clamp-2 text-xs break-words text-muted-foreground" title={alert.message}>
            {alert.message}
          </p>
        )}
      </div>
      <span className="mt-0.5 shrink-0 text-xs whitespace-nowrap text-muted-foreground">
        {formatRelativeTime(alert.firedAt, now)}
      </span>
    </li>
  )
}
