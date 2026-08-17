import { useMutation, useQuery } from '@tanstack/react-query'

import { api } from '@/lib/api'
import type { DatabasesResponse, QueryRequest, QueryResult } from '@/types/query'

export const DATABASES_QUERY_KEY = ['databases'] as const

/** Lists selectable databases for the worksheet's scope picker. */
export function useDatabases() {
  return useQuery({
    queryKey: DATABASES_QUERY_KEY,
    queryFn: async ({ signal }) => {
      const { data } = await api.get<DatabasesResponse>('/databases', { signal })
      return data.databases
    },
    // Database lists change rarely; a stale list is harmless.
    staleTime: 60_000,
    retry: 1,
  })
}

/**
 * Executes worksheet SQL. A mutation, not a query: executions are explicit
 * user actions that must never be retried or refetched automatically.
 */
export function useRunQuery() {
  return useMutation({
    mutationFn: async (request: QueryRequest) => {
      const { data } = await api.post<QueryResult>('/query', request)
      return data
    },
  })
}
