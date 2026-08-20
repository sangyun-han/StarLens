import { Boxes, ChevronRight, Database, HardDrive, Layers, TriangleAlert } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/ErrorState'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  BackendDistribution,
  SkewCard,
} from '@/features/storage/components/SkewBar'
import { useDatabases } from '@/hooks/useQueryRunner'
import { useStorageStatistic, useTableDetail, useTables } from '@/hooks/useStorage'
import { formatBytes, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { DatabaseStatistic, TableDetail } from '@/types/storage'

/**
 * The storage layer: catalog counts and tablet health per database, a table
 * browser, and per-table tablet/rowset/segment detail with data skew.
 */
export function StorageView() {
  const { t } = useTranslation()
  const [database, setDatabase] = useState<string | null>(null)
  const [table, setTable] = useState<string | null>(null)

  const statistic = useStorageStatistic()
  const { data: databases } = useDatabases()
  const tables = useTables(database)
  const detail = useTableDetail(database, table)

  if (statistic.isPending) return <StorageSkeleton />

  if (statistic.isError && !statistic.data) {
    return (
      <ErrorState
        title={t('storage.loadError')}
        error={statistic.error}
        onRetry={() => void statistic.refetch()}
        isRetrying={statistic.isFetching}
      />
    )
  }

  return (
    <div className="flex flex-col gap-6">
      {statistic.data && <StatisticSummary totals={statistic.data.totals} />}

      {statistic.data && <DatabaseHealthTable databases={statistic.data.databases} />}

      <Card size="sm">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="size-4 text-muted-foreground" />
            {t('storage.tables.title')}
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Select
            value={database ?? ''}
            onValueChange={(value) => {
              setDatabase(value)
              setTable(null)
            }}
          >
            <SelectTrigger className="w-64" aria-label={t('storage.tables.database')}>
              <Database className="size-4 shrink-0 text-muted-foreground" />
              <SelectValue placeholder={t('storage.tables.selectDatabase')} />
            </SelectTrigger>
            <SelectContent>
              {(databases ?? []).map((name) => (
                <SelectItem key={name} value={name}>
                  {name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {!database ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              {t('storage.tables.selectPrompt')}
            </p>
          ) : tables.isPending ? (
            <Skeleton className="h-32" />
          ) : tables.data && tables.data.tables.length > 0 ? (
            <TableBrowser
              tables={tables.data.tables}
              selected={table}
              onSelect={setTable}
            />
          ) : (
            <p className="py-6 text-center text-sm text-muted-foreground">
              {t('storage.tables.empty')}
            </p>
          )}
        </CardContent>
      </Card>

      {table && detail.isPending && <Skeleton className="h-72" />}
      {table && detail.data && <TableDetailPanel detail={detail.data} />}
    </div>
  )
}

function StatisticSummary({ totals }: { totals: DatabaseStatistic }) {
  const { t } = useTranslation()
  const unhealthy =
    totals.unhealthyTabletNum + totals.inconsistentTabletNum + totals.errorStateTabletNum

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Stat icon={Layers} label={t('storage.summary.tables')} value={formatNumber(totals.tableNum)} />
      <Stat icon={Boxes} label={t('storage.summary.tablets')} value={formatNumber(totals.tabletNum)} />
      <Stat
        icon={HardDrive}
        label={t('storage.summary.replicas')}
        value={formatNumber(totals.replicaNum)}
      />
      <Stat
        icon={TriangleAlert}
        label={t('storage.summary.unhealthy')}
        value={formatNumber(unhealthy)}
        hint={
          unhealthy > 0
            ? t('storage.summary.unhealthyHint')
            : t('storage.summary.allHealthy')
        }
        tone={unhealthy > 0 ? 'bad' : 'good'}
      />
    </div>
  )
}

function DatabaseHealthTable({ databases }: { databases: DatabaseStatistic[] }) {
  const { t } = useTranslation()

  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Boxes className="size-4 text-muted-foreground" />
          {t('storage.databases.title')}
        </CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('storage.databases.database')}</TableHead>
              <TableHead className="text-right">{t('storage.databases.tables')}</TableHead>
              <TableHead className="text-right">{t('storage.databases.tablets')}</TableHead>
              <TableHead className="text-right">{t('storage.databases.replicas')}</TableHead>
              <TableHead className="text-right">{t('storage.databases.unhealthy')}</TableHead>
              <TableHead className="text-right">{t('storage.databases.inconsistent')}</TableHead>
              <TableHead className="text-right">{t('storage.databases.cloning')}</TableHead>
              <TableHead className="text-right">{t('storage.databases.errorState')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {databases.map((db) => (
              <TableRow key={db.database}>
                <TableCell className="font-medium">{db.database}</TableCell>
                <NumCell value={db.tableNum} />
                <NumCell value={db.tabletNum} />
                <NumCell value={db.replicaNum} />
                <NumCell value={db.unhealthyTabletNum} alarming />
                <NumCell value={db.inconsistentTabletNum} alarming />
                {/* Cloning is transient repair traffic, not a fault. */}
                <NumCell value={db.cloningTabletNum} />
                <NumCell value={db.errorStateTabletNum} alarming />
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function NumCell({ value, alarming = false }: { value: number; alarming?: boolean }) {
  return (
    <TableCell
      className={cn(
        'text-right font-mono text-xs tabular-nums',
        alarming && value > 0 && 'font-semibold text-destructive',
      )}
    >
      {formatNumber(value)}
    </TableCell>
  )
}

function TableBrowser({
  tables,
  selected,
  onSelect,
}: {
  tables: import('@/types/storage').TableSummary[]
  selected: string | null
  onSelect: (name: string) => void
}) {
  const { t } = useTranslation()

  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('storage.tables.name')}</TableHead>
            <TableHead>{t('storage.tables.model')}</TableHead>
            <TableHead>{t('storage.tables.distribution')}</TableHead>
            <TableHead className="text-right">{t('storage.tables.buckets')}</TableHead>
            <TableHead className="text-right">{t('storage.tables.rows')}</TableHead>
            <TableHead className="text-right">{t('storage.tables.size')}</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {tables.map((table) => (
            <TableRow
              key={table.name}
              onClick={() => onSelect(table.name)}
              className={cn(
                'cursor-pointer',
                selected === table.name && 'bg-primary/5 hover:bg-primary/10',
              )}
            >
              <TableCell className="font-medium">{table.name}</TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {table.model || '—'}
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {table.distributeKey
                  ? `${table.distributeType ?? 'HASH'}(${table.distributeKey})`
                  : '—'}
              </TableCell>
              <TableCell className="text-right font-mono text-xs tabular-nums">
                {formatNumber(table.distributeBucket)}
              </TableCell>
              <TableCell className="text-right font-mono text-xs tabular-nums">
                {formatNumber(table.rows)}
              </TableCell>
              <TableCell className="text-right font-mono text-xs tabular-nums">
                {formatBytes(table.dataBytes)}
              </TableCell>
              <TableCell className="w-8 text-muted-foreground">
                <ChevronRight className="size-4" />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function TableDetailPanel({ detail }: { detail: TableDetail }) {
  const { t } = useTranslation()

  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <HardDrive className="size-4 text-muted-foreground" />
          {detail.table.database}.{detail.table.name}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-5">
        {detail.warnings?.map((warning) => (
          <p
            key={warning}
            className="rounded-md bg-warning/10 px-3 py-2 text-xs text-warning ring-1 ring-inset ring-warning/25"
          >
            {warning}
          </p>
        ))}

        <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
          <MiniStat label={t('storage.detail.tablets')} value={formatNumber(detail.tabletTotal)} />
          <MiniStat label={t('storage.detail.rowsets')} value={formatNumber(detail.rowsetTotal)} />
          <MiniStat label={t('storage.detail.segments')} value={formatNumber(detail.segmentTotal)} />
          <MiniStat
            label={t('storage.detail.maxRowsets')}
            value={formatNumber(detail.maxRowsetsPerTablet)}
            hint={t('storage.detail.maxRowsetsHint')}
          />
          <MiniStat
            label={t('storage.detail.abnormal')}
            value={formatNumber(detail.abnormalTablets)}
            alarming={detail.abnormalTablets > 0}
          />
        </div>

        <section className="flex flex-col gap-2">
          <h3 className="text-xs tracking-wide text-muted-foreground uppercase">
            {t('storage.detail.skewTitle')}
          </h3>
          <div className="grid gap-2 sm:grid-cols-2">
            <SkewCard
              title={t('storage.detail.skewTablets')}
              hint={t('storage.detail.skewTabletsHint')}
              rowsRatio={detail.skew.acrossTablets.rowsRatio}
              bytesRatio={detail.skew.acrossTablets.bytesRatio}
              skewed={detail.skew.acrossTablets.skewed}
            />
            <SkewCard
              title={t('storage.detail.skewBackends')}
              hint={t('storage.detail.skewBackendsHint')}
              rowsRatio={detail.skew.acrossBackends.rowsRatio}
              bytesRatio={detail.skew.acrossBackends.bytesRatio}
              skewed={detail.skew.acrossBackends.skewed}
            />
          </div>
        </section>

        {detail.backends.length > 0 && (
          <section className="flex flex-col gap-2">
            <h3 className="text-xs tracking-wide text-muted-foreground uppercase">
              {t('storage.detail.distribution')}
            </h3>
            <BackendDistribution backends={detail.backends} />
          </section>
        )}

        {detail.partitions.length > 0 && (
          <section className="flex flex-col gap-2">
            <h3 className="text-xs tracking-wide text-muted-foreground uppercase">
              {t('storage.detail.partitions')}
            </h3>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('storage.detail.partition')}</TableHead>
                    <TableHead className="text-right">{t('storage.tables.rows')}</TableHead>
                    <TableHead className="text-right">{t('storage.tables.size')}</TableHead>
                    <TableHead className="text-right">{t('storage.tables.buckets')}</TableHead>
                    <TableHead className="text-right">{t('storage.detail.compactionScore')}</TableHead>
                    <TableHead className="text-right">{t('storage.detail.balanced')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detail.partitions.map((partition) => (
                    <TableRow key={partition.name}>
                      <TableCell className="font-medium">{partition.name}</TableCell>
                      <TableCell className="text-right font-mono text-xs tabular-nums">
                        {formatNumber(partition.rows)}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs tabular-nums">
                        {formatBytes(partition.dataBytes)}
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs tabular-nums">
                        {formatNumber(partition.buckets)}
                      </TableCell>
                      <TableCell
                        className={cn(
                          'text-right font-mono text-xs tabular-nums',
                          // A high score means reads merge un-compacted rowsets.
                          (partition.maxCompactionScore ?? 0) > 100 &&
                            'font-semibold text-destructive',
                        )}
                      >
                        {partition.maxCompactionScore?.toFixed(1) ?? '—'}
                      </TableCell>
                      <TableCell className="text-right text-xs">
                        {partition.balanced === undefined ? (
                          '—'
                        ) : partition.balanced ? (
                          <span className="text-success">{t('storage.detail.yes')}</span>
                        ) : (
                          <span className="text-warning">{t('storage.detail.no')}</span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </section>
        )}
      </CardContent>
    </Card>
  )
}

function MiniStat({
  label,
  value,
  hint,
  alarming = false,
}: {
  label: string
  value: string
  hint?: string
  alarming?: boolean
}) {
  return (
    <div className="rounded-md bg-muted/60 px-2 py-1.5" title={hint}>
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <p
        className={cn(
          'font-mono text-sm tabular-nums',
          alarming ? 'font-semibold text-destructive' : 'text-foreground',
        )}
      >
        {value}
      </p>
    </div>
  )
}

function Stat({
  icon: Icon,
  label,
  value,
  hint,
  tone = 'neutral',
}: {
  icon: LucideIcon
  label: string
  value: string
  hint?: string
  tone?: 'neutral' | 'good' | 'bad'
}) {
  return (
    <Card size="sm">
      <CardContent className="flex items-start gap-3">
        <span
          className={cn(
            'flex size-8 shrink-0 items-center justify-center rounded-md',
            tone === 'good' && 'bg-success/10 text-success',
            tone === 'bad' && 'bg-destructive/10 text-destructive',
            tone === 'neutral' && 'bg-muted text-muted-foreground',
          )}
        >
          <Icon className="size-4" />
        </span>
        <div className="min-w-0">
          <p className="text-xs tracking-wide text-muted-foreground uppercase">{label}</p>
          <p
            className={cn(
              'truncate font-mono text-lg leading-tight font-semibold tabular-nums',
              tone === 'bad' ? 'text-destructive' : 'text-foreground',
            )}
          >
            {value}
          </p>
          {hint && <p className="truncate text-xs text-muted-foreground">{hint}</p>}
        </div>
      </CardContent>
    </Card>
  )
}

function StorageSkeleton() {
  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-20" />
        ))}
      </div>
      <Skeleton className="h-48" />
      <Skeleton className="h-64" />
    </div>
  )
}
