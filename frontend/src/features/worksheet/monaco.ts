/**
 * Monaco wiring for the SQL worksheet.
 *
 * @monaco-editor/react loads Monaco from a CDN by default; an operator tool
 * must work in air-gapped environments, so the editor core is bundled instead.
 * Only the editor API plus the SQL grammar are imported — not the full
 * `monaco-editor` entry with every language.
 *
 * Import specifiers go through the package's exports map ("./*" maps into
 * esm/vs/), matching the monaco-editor 0.5x layout where languages live under
 * languages/definitions/<lang>/register.js.
 *
 * This module is imported by the lazily-loaded worksheet route only, so none
 * of it lands in the main bundle.
 */
import { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor/editor/editor.api.js'

import 'monaco-editor/languages/definitions/sql/register.js'
import EditorWorker from 'monaco-editor/editor/editor.worker.start.js?worker'

declare global {
  interface Window {
    MonacoEnvironment?: monaco.Environment
  }
}

// SQL highlighting is tokenizer-only; the generic editor worker suffices.
self.MonacoEnvironment = {
  getWorker: () => new EditorWorker(),
}

loader.config({ monaco })

export { monaco }
