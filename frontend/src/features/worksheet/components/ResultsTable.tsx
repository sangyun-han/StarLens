import { useTranslation } from 'react-i18next'

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { QueryResult } from '@/types/query'

/** Dynamic-column result grid for ad-hoc statements. */
export function ResultsTable({ result }: { result: QueryResult }) {
  const { t } = useTranslation()

  if (result.rowCount === 0) {
    return (
      <p className="py-8 text-center text-sm text-muted-foreground">
        {t('worksheet.noRows')}
      </p>
    )
  }

  return (
    <div className="overflow-auto">
      <Table>
        <TableHeader className="sticky top-0 bg-card">
          <TableRow>
            {result.columns.map((column, index) => (
              <TableHead
                key={`${column.name}-${index}`}
                title={column.type || undefined}
                className="whitespace-nowrap"
              >
                {column.name}
                {column.type && (
                  <span className="ml-1.5 font-mono text-[10px] font-normal text-muted-foreground/70 uppercase">
                    {column.type}
                  </span>
                )}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {result.rows.map((row, rowIndex) => (
            <TableRow key={rowIndex}>
              {row.map((cell, cellIndex) => (
                <TableCell
                  key={cellIndex}
                  className="max-w-100 truncate font-mono text-xs whitespace-nowrap"
                  title={cell ?? undefined}
                >
                  {cell === null ? (
                    <span className="text-muted-foreground/60 italic">NULL</span>
                  ) : (
                    cell
                  )}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
