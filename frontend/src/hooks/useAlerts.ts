import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import type { AlertListResponse, AlertTestResponse } from '@/types/alert'

export const ALERTS_QUERY_KEY = ['alerts'] as const

/** Alert history changes at most once per evaluation tick; poll gently. */
export const ALERTS_REFRESH_INTERVAL_MS = 15_000

async function fetchAlerts(signal: AbortSignal): Promise<AlertListResponse> {
  const { data } = await api.get<AlertListResponse>('/alerts', { signal })
  return data
}

/** Reads the fired-alert history, newest first. */
export function useAlerts() {
  return useQuery({
    queryKey: ALERTS_QUERY_KEY,
    queryFn: ({ signal }) => fetchAlerts(signal),
    refetchInterval: ALERTS_REFRESH_INTERVAL_MS,
    placeholderData: (previous) => previous,
    retry: 1,
  })
}

/**
 * Fires a synthetic alert through every configured notifier so an operator can
 * verify a channel (e.g. a Slack webhook) before trusting it with incidents.
 */
export function useTestAlert() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post<AlertTestResponse>('/alerts/test')
      return data
    },
    // The test alert lands in the history; refresh it right away.
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ALERTS_QUERY_KEY })
    },
  })
}
