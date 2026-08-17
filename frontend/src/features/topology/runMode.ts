import type { TFunction } from 'i18next'

import type { RunMode } from '@/types/topology'

const KNOWN_RUN_MODES = new Set(['shared_data', 'shared_nothing', 'unknown'])

/**
 * Localized label for the deployment mode read from the FE `run_mode` config.
 * A mode this client does not know renders as itself rather than being masked.
 */
export function runModeLabel(t: TFunction, runMode: RunMode): string {
  return KNOWN_RUN_MODES.has(runMode) ? t(`runMode.${runMode}`) : runMode
}
