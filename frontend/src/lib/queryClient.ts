import { QueryClient } from '@tanstack/react-query'

/**
 * Shared query client. Defaults lean toward "operators want fresh data":
 * refetch when the tab regains focus, but do not hammer a struggling cluster
 * with retries.
 */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 5_000,
        retry: 1,
        refetchOnWindowFocus: true,
      },
    },
  })
}
