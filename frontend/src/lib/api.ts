import axios, { type AxiosError } from 'axios'

/**
 * In dev, Vite proxies `/api` to the Go server (see vite.config.ts), so the
 * browser stays same-origin. Point VITE_API_BASE_URL at a remote API to
 * override.
 */
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

/** The error envelope every StarLens endpoint returns. */
interface ApiErrorBody {
  error?: {
    code?: string
    message?: string
    detail?: string
  }
}

/**
 * A normalized transport/API failure. `detail` carries the underlying cause
 * (usually a StarRocks driver error) which is the actual answer an operator
 * needs, so the UI surfaces it rather than hiding it.
 */
export class ApiError extends Error {
  readonly code: string
  readonly detail?: string
  readonly status?: number

  constructor(
    message: string,
    options: { code: string; detail?: string; status?: number },
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = options.code
    this.detail = options.detail
    this.status = options.status
  }

  /** True when StarRocks — not StarLens — is the thing that is down. */
  get isClusterUnavailable(): boolean {
    return (
      this.code === 'starrocks_unavailable' || this.code === 'starrocks_unreachable'
    )
  }
}

function toApiError(error: AxiosError<ApiErrorBody>): ApiError {
  const body = error.response?.data?.error

  if (body) {
    return new ApiError(body.message ?? error.message, {
      code: body.code ?? 'api_error',
      detail: body.detail,
      status: error.response?.status,
    })
  }

  if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
    return new ApiError('The StarLens API did not respond in time.', {
      code: 'timeout',
      detail: error.message,
    })
  }

  if (!error.response) {
    return new ApiError('Cannot reach the StarLens API.', {
      code: 'network_error',
      detail: error.message,
    })
  }

  return new ApiError(error.message, {
    code: 'api_error',
    status: error.response.status,
  })
}

export const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 15_000,
  headers: { Accept: 'application/json' },
})

api.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiErrorBody>) => Promise.reject(toApiError(error)),
)
