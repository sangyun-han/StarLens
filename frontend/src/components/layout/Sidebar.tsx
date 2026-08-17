import { PanelLeftClose, PanelLeftOpen, Telescope } from 'lucide-react'
import { NavLink } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { NAV_ITEMS } from '@/config/navigation'
import { useTopology } from '@/hooks/useTopology'
import { cn } from '@/lib/utils'
import { useAppStore } from '@/store/useAppStore'

export function Sidebar() {
  const collapsed = useAppStore((state) => state.sidebarCollapsed)
  const toggleSidebar = useAppStore((state) => state.toggleSidebar)
  const { data } = useTopology()

  return (
    <aside
      className={cn(
        'sticky top-0 flex h-screen shrink-0 flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-200',
        collapsed ? 'w-16' : 'w-64',
      )}
    >
      <div
        className={cn(
          'flex h-14 items-center border-b border-sidebar-border',
          collapsed ? 'justify-center px-2' : 'gap-2.5 px-4',
        )}
      >
        <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
          <Telescope className="size-4.5" />
        </span>
        {!collapsed && (
          <span className="min-w-0">
            <span className="block truncate font-heading text-sm font-semibold text-foreground">
              StarLens
            </span>
            <span className="block truncate text-xs text-muted-foreground">
              StarRocks console
            </span>
          </span>
        )}
      </div>

      <nav className="flex-1 overflow-y-auto p-2" aria-label="Main">
        <ul className="flex flex-col gap-1">
          {NAV_ITEMS.map((item) => (
            <li key={item.to}>
              <NavLink
                to={item.to}
                title={collapsed ? item.label : undefined}
                className={({ isActive }) =>
                  cn(
                    'flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors',
                    'focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                    collapsed && 'justify-center px-0',
                    isActive
                      ? 'bg-primary/10 font-medium text-primary'
                      : 'text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
                  )
                }
              >
                <item.icon className="size-4.5 shrink-0" />
                {!collapsed && (
                  <>
                    <span className="min-w-0 flex-1 truncate">{item.label}</span>
                    {!item.available && (
                      <span className="rounded-sm bg-muted px-1.5 py-0.5 text-[10px] font-medium tracking-wide text-muted-foreground uppercase">
                        Soon
                      </span>
                    )}
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <div className="border-t border-sidebar-border p-2">
        {!collapsed && (
          <dl className="mb-2 grid grid-cols-2 gap-1 px-2.5 py-1.5 text-xs">
            <dt className="text-muted-foreground">Frontends</dt>
            <dd className="text-right font-mono tabular-nums text-foreground">
              {data ? `${data.summary.frontendAlive}/${data.summary.frontendTotal}` : '—'}
            </dd>
            {/* Show the layer(s) this cluster actually has. */}
            {(!data || data.summary.backendTotal > 0 || data.runMode !== 'shared_data') && (
              <>
                <dt className="text-muted-foreground">Backends</dt>
                <dd className="text-right font-mono tabular-nums text-foreground">
                  {data ? `${data.summary.backendAlive}/${data.summary.backendTotal}` : '—'}
                </dd>
              </>
            )}
            {data && (data.summary.computeTotal > 0 || data.runMode === 'shared_data') && (
              <>
                <dt className="text-muted-foreground">Compute</dt>
                <dd className="text-right font-mono tabular-nums text-foreground">
                  {`${data.summary.computeAlive}/${data.summary.computeTotal}`}
                </dd>
              </>
            )}
          </dl>
        )}
        <Button
          variant="ghost"
          size="sm"
          onClick={toggleSidebar}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className={cn('w-full text-muted-foreground', !collapsed && 'justify-start')}
        >
          {collapsed ? (
            <PanelLeftOpen className="size-4" />
          ) : (
            <>
              <PanelLeftClose className="size-4" />
              Collapse
            </>
          )}
        </Button>
      </div>
    </aside>
  )
}
