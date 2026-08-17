import { useQuery } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { useAppStore } from '@/store/useAppStore'
import type { Topology } from '@/types/topology'

export const TOPOLOGY_QUERY_KEY = ['cluster', 'topology'] as const

/** Poll interval for cluster membership. SHOW FRONTENDS/BACKENDS is cheap. */
export const TOPOLOGY_REFRESH_INTERVAL_MS = 10_000

async function fetchTopology(signal: AbortSignal): Promise<Topology> {
  const { data } = await api.get<Topology>('/cluster/topology', { signal })
  return data
}

/**
 * Reads cluster topology. Every caller shares one cache entry, so the header
 * summary and the topology page never disagree or double-fetch.
 */
export function useTopology() {
  const autoRefresh = useAppStore((state) => state.autoRefresh)

  return useQuery({
    queryKey: TOPOLOGY_QUERY_KEY,
    queryFn: ({ signal }) => fetchTopology(signal),
    refetchInterval: autoRefresh ? TOPOLOGY_REFRESH_INTERVAL_MS : false,
    // Keep the previous snapshot on screen while refetching so the node grid
    // does not flash back to skeletons every poll.
    placeholderData: (previous) => previous,
    staleTime: 5_000,
    // A down cluster is an expected state, not a transport glitch worth retrying.
    retry: 1,
  })
}
