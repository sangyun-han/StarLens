import { RefreshCw, TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { ApiError } from '@/lib/api'

interface ErrorStateProps {
  title?: string
  error: unknown
  onRetry?: () => void
  isRetrying?: boolean
}

/**
 * Failure card for a whole page region. It shows the underlying cause verbatim,
 * because "dial tcp 10.0.0.1:9030: connection refused" is the answer an operator
 * is looking for. API-provided messages stay untranslated by design — they are
 * operator-facing diagnostics from the backend; the guidance hints are ours and
 * localized.
 */
export function ErrorState({
  title,
  error,
  onRetry,
  isRetrying = false,
}: ErrorStateProps) {
  const { t } = useTranslation()
  const { message, detail, hint } = describe(t, error)
  title ??= t('errors.defaultTitle')

  return (
    <Card className="border-destructive/30 ring-destructive/20">
      <CardContent className="flex flex-col items-start gap-3">
        <div className="flex items-start gap-3">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-destructive/10 text-destructive">
            <TriangleAlert className="size-4" />
          </span>
          <div className="min-w-0">
            <p className="font-heading text-sm font-semibold text-foreground">
              {title}
            </p>
            <p className="text-sm text-muted-foreground">{message}</p>
          </div>
        </div>

        {detail && (
          <pre className="w-full overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs text-foreground">
            {detail}
          </pre>
        )}

        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}

        {onRetry && (
          <Button variant="outline" size="sm" onClick={onRetry} disabled={isRetrying}>
            <RefreshCw className={isRetrying ? 'animate-spin' : undefined} />
            {t('common.retry')}
          </Button>
        )}
      </CardContent>
    </Card>
  )
}

function describe(
  t: TFunction,
  error: unknown,
): {
  message: string
  detail?: string
  hint?: string
} {
  if (error instanceof ApiError) {
    return {
      message: error.message,
      detail: error.detail,
      hint: hintFor(t, error.code),
    }
  }
  if (error instanceof Error) return { message: error.message }
  return { message: t('errors.unexpected') }
}

function hintFor(t: TFunction, code: string): string | undefined {
  switch (code) {
    case 'starrocks_unavailable':
    case 'starrocks_unreachable':
      return t('errors.hints.starrocksUnavailable')
    case 'network_error':
      return t('errors.hints.networkError')
    case 'timeout':
      return t('errors.hints.timeout')
    default:
      return undefined
  }
}
