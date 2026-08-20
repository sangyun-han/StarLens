import { RotateCcw, Settings2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useAlertConfig, useTestAlert, useUpdateAlertConfig } from '@/hooks/useAlerts'
import { ApiError } from '@/lib/api'
import type { AlertConfigPatch, AlertConfigView, WebhookFormat } from '@/types/alert'

interface FormState {
  enabled: boolean
  pollInterval: string
  cooldown: string
  /** Write-only: empty means "keep the current webhook untouched". */
  webhookUrl: string
  clearWebhook: boolean
  webhookFormat: WebhookFormat
  /** Percent in the UI; the API speaks fractions. */
  errorRowsPercent: string
  errorRowsMinTotal: string
  maxOffsetLag: string
  maxJournalLag: string
}

function toFormState(view: AlertConfigView): FormState {
  return {
    enabled: view.config.enabled,
    pollInterval: view.config.pollInterval,
    cooldown: view.config.cooldown,
    webhookUrl: '',
    clearWebhook: false,
    webhookFormat: view.config.webhookFormat,
    errorRowsPercent: String(Math.round(view.config.errorRowsRatio * 10000) / 100),
    errorRowsMinTotal: String(view.config.errorRowsMinTotal),
    maxOffsetLag: String(view.config.maxOffsetLag),
    maxJournalLag: String(view.config.maxJournalLag),
  }
}

function toPatch(form: FormState): AlertConfigPatch {
  const patch: AlertConfigPatch = {
    enabled: form.enabled,
    pollInterval: form.pollInterval.trim(),
    cooldown: form.cooldown.trim(),
    webhookFormat: form.webhookFormat,
    errorRowsRatio: Number(form.errorRowsPercent) / 100,
    errorRowsMinTotal: Number(form.errorRowsMinTotal),
    maxOffsetLag: Number(form.maxOffsetLag),
    maxJournalLag: Number(form.maxJournalLag),
  }
  // The URL is write-only: only send it when the operator typed a new one or
  // explicitly asked to remove the webhook.
  if (form.clearWebhook) {
    patch.webhookUrl = ''
  } else if (form.webhookUrl.trim() !== '') {
    patch.webhookUrl = form.webhookUrl.trim()
  }
  return patch
}

/**
 * Runtime alert configuration: environment variables are the defaults, saves
 * become file-persisted overrides applied without a restart. Read-only when
 * the server runs with ALERT_CONFIG_UI=false.
 */
export function AlertSettingsDialog() {
  const { t } = useTranslation()
  const { data: view } = useAlertConfig()
  const updateConfig = useUpdateAlertConfig()
  const testAlert = useTestAlert()

  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<FormState | null>(null)
  const [outcome, setOutcome] = useState<string | null>(null)

  const editable = view?.editable ?? false
  const busy = updateConfig.isPending || testAlert.isPending

  const openChange = (next: boolean) => {
    setOpen(next)
    setOutcome(null)
    updateConfig.reset()
    setForm(next && view ? toFormState(view) : null)
  }

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => (prev ? { ...prev, [key]: value } : prev))
  }

  const save = (thenTest: boolean) => {
    if (!form) return
    setOutcome(null)
    updateConfig.mutate(toPatch(form), {
      onSuccess: (saved) => {
        setForm(toFormState(saved))
        if (!thenTest) {
          setOutcome(t('alertSettings.saved'))
          return
        }
        testAlert.mutate(undefined, {
          onSuccess: (response) => {
            const failed = Object.entries(response.results).filter(([, r]) => r !== 'ok')
            setOutcome(
              failed.length === 0
                ? t('alertSettings.savedAndDelivered')
                : t('alerts.deliveryFailed', {
                    list: failed.map(([name, err]) => `${name} (${err})`).join(', '),
                  }),
            )
          },
          onError: () => setOutcome(t('alerts.apiUnreachable')),
        })
      },
    })
  }

  const resetToEnv = () => {
    setOutcome(null)
    updateConfig.mutate(
      { reset: true },
      {
        onSuccess: (saved) => {
          setForm(toFormState(saved))
          setOutcome(t('alertSettings.resetDone'))
        },
      },
    )
  }

  const error = updateConfig.error instanceof ApiError ? updateConfig.error : null

  return (
    <Dialog open={open} onOpenChange={openChange}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t('alertSettings.title')}>
          <Settings2 className="size-4 text-muted-foreground" />
        </Button>
      </DialogTrigger>

      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('alertSettings.title')}</DialogTitle>
          <DialogDescription>
            {editable ? t('alertSettings.description') : t('alertSettings.readOnly')}
          </DialogDescription>
        </DialogHeader>

        {form && view && (
          <div className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-4">
              <Label htmlFor="alert-enabled">{t('alertSettings.enabled')}</Label>
              <Switch
                id="alert-enabled"
                checked={form.enabled}
                onCheckedChange={(checked) => set('enabled', checked === true)}
                disabled={!editable}
              />
            </div>

            <Field
              id="alert-webhook"
              label={t('alertSettings.webhookUrl')}
              hint={
                form.clearWebhook
                  ? t('alertSettings.webhookWillBeRemoved')
                  : view.config.webhookConfigured
                    ? t('alertSettings.webhookConfigured', { hint: view.config.webhookHint })
                    : t('alertSettings.webhookNotConfigured')
              }
            >
              <div className="flex gap-2">
                <Input
                  id="alert-webhook"
                  type="url"
                  placeholder="https://hooks.slack.com/services/…"
                  value={form.webhookUrl}
                  onChange={(e) => {
                    set('webhookUrl', e.target.value)
                    set('clearWebhook', false)
                  }}
                  disabled={!editable || form.clearWebhook}
                />
                {view.config.webhookConfigured && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={!editable}
                    onClick={() => {
                      set('clearWebhook', !form.clearWebhook)
                      set('webhookUrl', '')
                    }}
                  >
                    {form.clearWebhook
                      ? t('alertSettings.keepWebhook')
                      : t('alertSettings.removeWebhook')}
                  </Button>
                )}
              </div>
            </Field>

            <Field id="alert-format" label={t('alertSettings.webhookFormat')}>
              <Select
                value={form.webhookFormat}
                onValueChange={(value) => set('webhookFormat', value as WebhookFormat)}
                disabled={!editable}
              >
                <SelectTrigger id="alert-format" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="slack">Slack ({'{"text": …}'})</SelectItem>
                  <SelectItem value="generic">{t('alertSettings.formatGeneric')}</SelectItem>
                </SelectContent>
              </Select>
            </Field>

            <div className="grid grid-cols-2 gap-3">
              <Field id="alert-interval" label={t('alertSettings.pollInterval')}>
                <Input
                  id="alert-interval"
                  value={form.pollInterval}
                  onChange={(e) => set('pollInterval', e.target.value)}
                  placeholder="30s"
                  disabled={!editable}
                />
              </Field>
              <Field id="alert-cooldown" label={t('alertSettings.cooldown')}>
                <Input
                  id="alert-cooldown"
                  value={form.cooldown}
                  onChange={(e) => set('cooldown', e.target.value)}
                  placeholder="10m"
                  disabled={!editable}
                />
              </Field>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <Field id="alert-journal-lag" label={t('alertSettings.maxJournalLag')}>
                <Input
                  id="alert-journal-lag"
                  type="number"
                  min="0"
                  value={form.maxJournalLag}
                  onChange={(e) => set('maxJournalLag', e.target.value)}
                  disabled={!editable}
                />
              </Field>
              <Field id="alert-lag" label={t('alertSettings.maxOffsetLag')}>
                <Input
                  id="alert-lag"
                  type="number"
                  min="0"
                  value={form.maxOffsetLag}
                  onChange={(e) => set('maxOffsetLag', e.target.value)}
                  disabled={!editable}
                />
              </Field>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <Field id="alert-ratio" label={t('alertSettings.errorRowsPercent')}>
                <Input
                  id="alert-ratio"
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  value={form.errorRowsPercent}
                  onChange={(e) => set('errorRowsPercent', e.target.value)}
                  disabled={!editable}
                />
              </Field>
              <Field id="alert-min-total" label={t('alertSettings.errorRowsMinTotal')}>
                <Input
                  id="alert-min-total"
                  type="number"
                  min="0"
                  value={form.errorRowsMinTotal}
                  onChange={(e) => set('errorRowsMinTotal', e.target.value)}
                  disabled={!editable}
                />
              </Field>
            </div>
            <p className="text-xs text-muted-foreground">{t('alertSettings.zeroDisables')}</p>

            {view.overridden.length > 0 && (
              <p className="text-xs text-muted-foreground">
                {t('alertSettings.overriddenNote', {
                  fields: view.overridden.join(', '),
                })}
              </p>
            )}

            {error && (
              <p className="rounded-md bg-destructive/10 px-3 py-2 text-xs break-words text-destructive">
                {error.detail || error.message}
              </p>
            )}
            {outcome && (
              <p className="rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">
                {outcome}
              </p>
            )}
          </div>
        )}

        {editable && (
          <DialogFooter className="gap-2 sm:justify-between">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={resetToEnv}
              disabled={busy || (view?.overridden.length ?? 0) === 0}
              className="text-muted-foreground"
            >
              <RotateCcw className="size-3.5" />
              {t('alertSettings.resetToEnv')}
            </Button>
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={() => save(false)} disabled={busy}>
                {t('alertSettings.save')}
              </Button>
              <Button type="button" onClick={() => save(true)} disabled={busy}>
                {t('alertSettings.saveAndTest')}
              </Button>
            </div>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}

function Field({
  id,
  label,
  hint,
  children,
}: {
  id: string
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}
