import { ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { JobStateBadge } from '@/features/loads/components/JobStateBadge'
import { formatNumber, formatPercent } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { RoutineLoadJob } from '@/types/routineload'

/** Error ratio at which the error-rows cell turns red (matches backend default). */
const ERROR_RATIO_HIGHLIGHT = 0.01

export function JobsTable({ jobs }: { jobs: RoutineLoadJob[] }) {
  const { t } = useTranslation()

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('loads.table.job')}</TableHead>
            <TableHead>{t('loads.table.state')}</TableHead>
            <TableHead>{t('loads.table.source')}</TableHead>
            <TableHead className="text-right">{t('loads.table.tasks')}</TableHead>
            <TableHead className="text-right">{t('loads.table.loadedRows')}</TableHead>
            <TableHead className="text-right">{t('loads.table.errorRows')}</TableHead>
            <TableHead className="text-right">{t('loads.table.lag')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {jobs.map((job) => (
            <JobRow key={job.id || `${job.database}.${job.name}`} job={job} />
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function JobRow({ job }: { job: RoutineLoadJob }) {
  const { t } = useTranslation()
  const stats = job.statistics
  const errorRatio =
    stats && stats.totalRows > 0 ? stats.errorRows / stats.totalRows : 0
  const reason = job.reasonOfStateChanged || job.otherMsg

  return (
    <>
      <TableRow className={cn(job.state === 'CANCELLED' && 'bg-destructive/[0.03]')}>
        <TableCell>
          <p className="font-medium text-foreground">{job.name}</p>
          <p className="font-mono text-xs text-muted-foreground">
            {job.database}.{job.table}
          </p>
        </TableCell>
        <TableCell>
          <JobStateBadge state={job.state} />
        </TableCell>
        <TableCell className="text-xs text-muted-foreground">
          {job.dataSourceType || '—'}
        </TableCell>
        <TableCell className="text-right font-mono text-xs tabular-nums">
          {formatNumber(job.currentTaskNum)}
        </TableCell>
        <TableCell className="text-right font-mono text-xs tabular-nums">
          {formatNumber(stats?.loadedRows)}
        </TableCell>
        <TableCell
          className={cn(
            'text-right font-mono text-xs tabular-nums',
            stats &&
              stats.errorRows > 0 &&
              (errorRatio > ERROR_RATIO_HIGHLIGHT
                ? 'font-semibold text-destructive'
                : 'text-warning'),
          )}
          title={
            stats && stats.totalRows > 0
              ? t('loads.table.errorRowsTooltip', {
                  percent: formatPercent(errorRatio * 100, 2),
                  total: formatNumber(stats.totalRows),
                })
              : undefined
          }
        >
          {formatNumber(stats?.errorRows)}
        </TableCell>
        <TableCell
          className="text-right font-mono text-xs tabular-nums"
          title={job.offsetLag !== undefined ? t('loads.table.lagTooltip') : undefined}
        >
          {job.offsetLag !== undefined ? formatNumber(job.offsetLag) : '—'}
        </TableCell>
      </TableRow>

      {/* Unhealthy jobs get a detail row: the reason is the actionable part. */}
      {(job.state === 'PAUSED' || job.state === 'CANCELLED') && reason && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={7} className="py-2">
            <div className="flex flex-col gap-1 rounded-md bg-destructive/5 px-3 py-2">
              <p className="font-mono text-xs break-words text-destructive">{reason}</p>
              {job.errorLogUrls && (
                <a
                  href={job.errorLogUrls.split(',')[0]}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex w-fit items-center gap-1 text-xs text-primary hover:underline"
                >
                  <ExternalLink className="size-3" />
                  {t('common.errorLog')}
                </a>
              )}
            </div>
          </TableCell>
        </TableRow>
      )}
    </>
  )
}
