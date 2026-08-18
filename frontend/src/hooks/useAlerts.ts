import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import type {
  AlertConfigPatch,
  AlertConfigView,
  AlertListResponse,
  AlertTestResponse,
} from '@/types/alert'

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

export const ALERT_CONFIG_QUERY_KEY = ['alerts', 'config'] as const

/** Reads the effective alerting configuration (webhook URL masked). */
export function useAlertConfig() {
  return useQuery({
    queryKey: ALERT_CONFIG_QUERY_KEY,
    queryFn: async ({ signal }) => {
      const { data } = await api.get<AlertConfigView>('/alerts/config', { signal })
      return data
    },
    staleTime: 30_000,
    retry: 1,
  })
}

/** Persists an alert configuration patch; changes apply without a restart. */
export function useUpdateAlertConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (patch: AlertConfigPatch) => {
      const { data } = await api.put<AlertConfigView>('/alerts/config', patch)
      return data
    },
    onSuccess: (view) => {
      queryClient.setQueryData(ALERT_CONFIG_QUERY_KEY, view)
    },
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
