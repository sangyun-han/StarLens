import Editor from '@monaco-editor/react'
import { CircleAlert, Database, Loader, Play } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from '@/components/ui/resizable'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ResultsTable } from '@/features/worksheet/components/ResultsTable'
import { QueryProfile } from '@/features/worksheet/components/QueryProfile'
import { monaco } from '@/features/worksheet/monaco'
import { useDatabases, useRunQuery } from '@/hooks/useQueryRunner'
import { ApiError } from '@/lib/api'
import { formatNumber } from '@/lib/format'
import { useAppStore } from '@/store/useAppStore'

/** Sentinel for "no USE, run against the connection's default database". */
const DEFAULT_DB = '__default__'

/**
 * The SQL worksheet: a Monaco editor on top, results and the execution profile
 * below, with a database scope picker. Cmd/Ctrl+Enter runs the statement.
 */
export default function SqlWorksheet() {
  const { t } = useTranslation()
  const sql = useAppStore((state) => state.worksheetSql)
  const setSql = useAppStore((state) => state.setWorksheetSql)
  const database = useAppStore((state) => state.currentDatabase)
  const setDatabase = useAppStore((state) => state.setCurrentDatabase)

  const { data: databases } = useDatabases()
  const runQuery = useRunQuery()
  const [activeTab, setActiveTab] = useState('results')

  const run = useCallback(() => {
    const statement = useAppStore.getState().worksheetSql
    if (!statement.trim() || runQuery.isPending) return
    runQuery.mutate(
      {
        sql: statement,
        database: useAppStore.getState().currentDatabase ?? undefined,
      },
      { onSuccess: () => setActiveTab('results') },
    )
  }, [runQuery])

  // The editor keydown handler must always see the latest run closure.
  const runRef = useRef(run)
  useEffect(() => {
    runRef.current = run
  }, [run])

  return (
    <div className="flex h-[calc(100vh-8.5rem)] min-h-120 flex-col gap-3">
      <div className="flex items-center gap-2">
        <Button onClick={run} disabled={runQuery.isPending || !sql.trim()}>
          {runQuery.isPending ? (
            <Loader className="size-4 animate-spin" />
          ) : (
            <Play className="size-4" />
          )}
          {runQuery.isPending ? t('worksheet.running') : t('worksheet.run')}
        </Button>

        <Select
          value={database ?? DEFAULT_DB}
          onValueChange={(value) => setDatabase(value === DEFAULT_DB ? null : value)}
        >
          <SelectTrigger className="w-56" aria-label={t('worksheet.database')}>
            <Database className="size-4 shrink-0 text-muted-foreground" />
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={DEFAULT_DB}>{t('worksheet.defaultDatabase')}</SelectItem>
            {(databases ?? []).map((name) => (
              <SelectItem key={name} value={name}>
                {name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <span className="ml-auto hidden text-xs text-muted-foreground sm:inline">
          {t('worksheet.shortcutHint')}
        </span>
      </div>

      <ResizablePanelGroup
        orientation="vertical"
        className="min-h-0 flex-1 overflow-hidden rounded-xl ring-1 ring-foreground/10"
      >
        <ResizablePanel defaultSize={45} minSize={20}>
          <div className="h-full bg-card pt-2">
            <Editor
              language="sql"
              theme="vs"
              value={sql}
              onChange={(value) => setSql(value ?? '')}
              onMount={(editor) => {
                editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () =>
                  runRef.current(),
                )
              }}
              options={{
                minimap: { enabled: false },
                fontSize: 13,
                fontFamily:
                  "ui-monospace, 'SFMono-Regular', 'JetBrains Mono', 'Menlo', 'Consolas', monospace",
                lineNumbersMinChars: 3,
                scrollBeyondLastLine: false,
                automaticLayout: true,
                padding: { top: 4 },
                renderLineHighlight: 'none',
                overviewRulerLanes: 0,
              }}
            />
          </div>
        </ResizablePanel>

        <ResizableHandle withHandle />

        <ResizablePanel defaultSize={55} minSize={20}>
          <div className="flex h-full flex-col bg-card">
            <ResultsArea
              activeTab={activeTab}
              onTabChange={setActiveTab}
              runQuery={runQuery}
            />
          </div>
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  )
}

function ResultsArea({
  activeTab,
  onTabChange,
  runQuery,
}: {
  activeTab: string
  onTabChange: (tab: string) => void
  runQuery: ReturnType<typeof useRunQuery>
}) {
  const { t } = useTranslation()

  if (runQuery.isPending) {
    return (
      <div className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
        <Loader className="size-4 animate-spin" />
        {t('worksheet.running')}
      </div>
    )
  }

  if (runQuery.isError) {
    return <QueryError error={runQuery.error} />
  }

  const result = runQuery.data
  if (!result) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
        {t('worksheet.emptyPrompt')}
      </div>
    )
  }

  return (
    <Tabs value={activeTab} onValueChange={onTabChange} className="flex min-h-0 flex-1 flex-col gap-0">
      <div className="flex items-center gap-3 border-b border-border px-3 pt-1">
        <TabsList>
          <TabsTrigger value="results">{t('worksheet.results')}</TabsTrigger>
          <TabsTrigger value="profile">{t('worksheet.profile.title')}</TabsTrigger>
        </TabsList>
        <span className="ml-auto font-mono text-xs text-muted-foreground tabular-nums">
          {t('worksheet.resultSummary', {
            rows: formatNumber(result.rowCount),
            elapsed: formatNumber(result.elapsedMs),
          })}
        </span>
      </div>

      {result.truncated && (
        <p className="border-b border-border bg-warning/10 px-3 py-1.5 text-xs text-warning">
          {t('worksheet.truncatedNotice', { count: result.maxRows })}
        </p>
      )}

      <TabsContent value="results" className="min-h-0 flex-1 overflow-auto">
        <ResultsTable result={result} />
      </TabsContent>
      <TabsContent value="profile" className="min-h-0 flex-1 overflow-auto px-3">
        <QueryProfile result={result} />
      </TabsContent>
    </Tabs>
  )
}

/** SQL errors inline where results would be — the message is the answer. */
function QueryError({ error }: { error: unknown }) {
  const { t } = useTranslation()
  const apiError = error instanceof ApiError ? error : null

  return (
    <div className="flex flex-1 items-start overflow-auto p-3">
      <Card className="w-full border-destructive/30 ring-destructive/20">
        <CardContent className="flex flex-col gap-2">
          <p className="flex items-center gap-2 text-sm font-medium text-destructive">
            <CircleAlert className="size-4 shrink-0" />
            {apiError?.message ?? t('errors.unexpected')}
          </p>
          {apiError?.detail && (
            <pre className="overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap text-foreground">
              {apiError.detail}
            </pre>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
