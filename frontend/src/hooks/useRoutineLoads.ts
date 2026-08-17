import { useQuery } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { useAppStore } from '@/store/useAppStore'
import type { RoutineLoadSnapshot } from '@/types/routineload'

export const ROUTINE_LOADS_QUERY_KEY = ['loads', 'routine'] as const

/** Poll interval for routine load jobs; ingestion state changes fast. */
export const ROUTINE_LOADS_REFRESH_INTERVAL_MS = 10_000

async function fetchRoutineLoads(signal: AbortSignal): Promise<RoutineLoadSnapshot> {
  const { data } = await api.get<RoutineLoadSnapshot>('/loads/routine', { signal })
  return data
}

/** Reads every routine load job across databases, with summary and warnings. */
export function useRoutineLoads() {
  const autoRefresh = useAppStore((state) => state.autoRefresh)

  return useQuery({
    queryKey: ROUTINE_LOADS_QUERY_KEY,
    queryFn: ({ signal }) => fetchRoutineLoads(signal),
    refetchInterval: autoRefresh ? ROUTINE_LOADS_REFRESH_INTERVAL_MS : false,
    // Keep the previous snapshot on screen while refetching.
    placeholderData: (previous) => previous,
    staleTime: 5_000,
    retry: 1,
  })
}
