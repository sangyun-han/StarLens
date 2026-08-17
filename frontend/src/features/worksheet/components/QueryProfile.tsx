import { useTranslation } from 'react-i18next'

import { formatNumber } from '@/lib/format'
import type { QueryResult } from '@/types/query'

/**
 * Execution facts for the last run. ElapsedMs is the API server's wall clock —
 * an operator-facing approximation, not the engine's internal profile (that
 * lands here once profile collection is wired up).
 */
export function QueryProfile({ result }: { result: QueryResult }) {
  const { t } = useTranslation()

  return (
    <div className="flex flex-col gap-4 py-2">
      <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-4">
        <ProfileFact
          label={t('worksheet.profile.elapsed')}
          value={`${formatNumber(result.elapsedMs)} ms`}
        />
        <ProfileFact
          label={t('worksheet.profile.rows')}
          value={
            result.truncated
              ? t('worksheet.profile.rowsTruncated', {
                  count: result.rowCount,
                })
              : formatNumber(result.rowCount)
          }
        />
        <ProfileFact
          label={t('worksheet.profile.rowLimit')}
          value={formatNumber(result.maxRows)}
        />
        <ProfileFact
          label={t('worksheet.profile.database')}
          value={result.database || t('worksheet.defaultDatabase')}
        />
      </dl>

      <div>
        <p className="mb-1 text-xs tracking-wide text-muted-foreground uppercase">
          {t('worksheet.profile.statement')}
        </p>
        <pre className="overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap text-foreground">
          {result.statement}
        </pre>
      </div>
    </div>
  )
}

function ProfileFact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs tracking-wide text-muted-foreground uppercase">{label}</dt>
      <dd className="font-mono text-sm tabular-nums text-foreground">{value}</dd>
    </div>
  )
}
