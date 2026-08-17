import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AppState {
  /** Sidebar collapsed to icons only. */
  sidebarCollapsed: boolean
  toggleSidebar: () => void
  setSidebarCollapsed: (collapsed: boolean) => void

  /** Database scope applied to the worksheet and lineage views. */
  currentDatabase: string | null
  setCurrentDatabase: (database: string | null) => void

  /** Whether cluster queries poll on an interval. */
  autoRefresh: boolean
  toggleAutoRefresh: () => void
}

/**
 * Global UI state. Server data lives in TanStack Query, so this store holds
 * only what the user chose — which is exactly what is worth persisting across
 * reloads.
 */
export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      toggleSidebar: () =>
        set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
      setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),

      currentDatabase: null,
      setCurrentDatabase: (database) => set({ currentDatabase: database }),

      autoRefresh: true,
      toggleAutoRefresh: () =>
        set((state) => ({ autoRefresh: !state.autoRefresh })),
    }),
    { name: 'starlens.ui' },
  ),
)
