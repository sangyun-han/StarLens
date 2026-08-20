import { useQuery } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { useAppStore } from '@/store/useAppStore'
import type { StorageStatistic, TableDetail, TableList } from '@/types/storage'

export const STORAGE_STATISTIC_QUERY_KEY = ['storage', 'statistic'] as const

/** Catalog counts and tablet health per database. */
export function useStorageStatistic() {
  const autoRefresh = useAppStore((state) => state.autoRefresh)

  return useQuery({
    queryKey: STORAGE_STATISTIC_QUERY_KEY,
    queryFn: async ({ signal }) => {
      const { data } = await api.get<StorageStatistic>('/storage/statistic', { signal })
      return data
    },
    // Tablet health moves on repair/rebalance timescales, not per second.
    refetchInterval: autoRefresh ? 30_000 : false,
    placeholderData: (previous) => previous,
    retry: 1,
  })
}

/** Lists one database's tables. Disabled until a database is chosen. */
export function useTables(database: string | null) {
  return useQuery({
    queryKey: ['storage', 'tables', database] as const,
    queryFn: async ({ signal }) => {
      const { data } = await api.get<TableList>('/storage/tables', {
        params: { database },
        signal,
      })
      return data
    },
    enabled: Boolean(database),
    staleTime: 30_000,
    retry: 1,
  })
}

/**
 * Partition, tablet, rowset/segment and skew detail for one table. Scoped by
 * table on purpose: the backing view is expensive to scan cluster-wide.
 */
export function useTableDetail(database: string | null, table: string | null) {
  return useQuery({
    queryKey: ['storage', 'table', database, table] as const,
    queryFn: async ({ signal }) => {
      const { data } = await api.get<TableDetail>(
        `/storage/tables/${encodeURIComponent(database!)}/${encodeURIComponent(table!)}`,
        { signal },
      )
      return data
    },
    enabled: Boolean(database && table),
    staleTime: 15_000,
    retry: 1,
  })
}
