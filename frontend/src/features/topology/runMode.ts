import type { RunMode } from '@/types/topology'

/** Human labels for the deployment mode read from the FE `run_mode` config. */
const RUN_MODE_LABELS: Record<string, string> = {
  shared_data: 'Shared-data',
  shared_nothing: 'Shared-nothing',
  unknown: 'Unknown mode',
}

export function runModeLabel(runMode: RunMode): string {
  return RUN_MODE_LABELS[runMode] ?? runMode
}
