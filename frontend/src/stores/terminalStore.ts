import { create } from 'zustand'

export interface AssetInfo {
  id: number
  name: string
  type: string
  host: string
  port: number
  username: string
  auth_type: string
  encoding: string
  [key: string]: any
}

export interface TerminalTab {
  id: string
  asset: AssetInfo
  connected: boolean
  lastOutput: string
}

export interface SplitPane {
  direction: 'horizontal' | 'vertical'
  children: (string | SplitPane)[]
}

interface TerminalState {
  tabs: TerminalTab[]
  activeTabId: string | null
  splitPanes: SplitPane | null
  addTab: (asset: AssetInfo) => string
  removeTab: (id: string) => void
  setActiveTab: (id: string) => void
  setConnected: (id: string, connected: boolean) => void
  appendOutput: (id: string, data: string) => void
  clearOutput: (id: string) => void
}

export const useTerminalStore = create<TerminalState>((set) => ({
  tabs: [],
  activeTabId: null,
  splitPanes: null,

  addTab: (asset) => {
    const id = `term-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`
    set((state) => ({
      tabs: [...state.tabs, { id, asset, connected: false, lastOutput: '' }],
      activeTabId: id,
    }))
    return id
  },

  removeTab: (id) =>
    set((state) => {
      const tabs = state.tabs.filter((t) => t.id !== id)
      const activeTabId = state.activeTabId === id
        ? (tabs.length > 0 ? tabs[tabs.length - 1].id : null)
        : state.activeTabId
      return { tabs, activeTabId }
    }),

  setActiveTab: (id) => set({ activeTabId: id }),

  setConnected: (id, connected) =>
    set((state) => ({
      tabs: state.tabs.map((t) => (t.id === id ? { ...t, connected } : t)),
    })),

  appendOutput: (id, data) =>
    set((state) => ({
      tabs: state.tabs.map((t) =>
        t.id === id ? { ...t, lastOutput: (t.lastOutput + data).slice(-50000) } : t
      ),
    })),

  clearOutput: (id) =>
    set((state) => ({
      tabs: state.tabs.map((t) => (t.id === id ? { ...t, lastOutput: '' } : t)),
    })),
}))
