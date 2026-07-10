import { useCallback, useRef, useEffect } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { useTerminalStore } from '../stores/terminalStore'

/**
 * useTerminal - Custom hook for managing xterm.js terminal instances
 * Handles terminal lifecycle, fit, search, and WebSocket connection
 */
export function useTerminal(containerRef: React.RefObject<HTMLDivElement | null>) {
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const searchRef = useRef<SearchAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectAttempt = useRef(0)
  const maxReconnectAttempts = 5

  const tabs = useTerminalStore((s) => s.tabs)
  const addTab = useTerminalStore((s) => s.addTab)
  const removeTab = useTerminalStore((s) => s.removeTab)
  const setConnected = useTerminalStore((s) => s.setConnected)
  const appendOutput = useTerminalStore((s) => s.appendOutput)

  const initTerminal = useCallback((cols = 80, rows = 24) => {
    if (!containerRef.current) return null

    const term = new Terminal({
      cols,
      rows,
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
        cursor: '#d4d4d4',
        selectionBackground: '#264f78',
      },
    })

    const fitAddon = new FitAddon()
    const searchAddon = new SearchAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(searchAddon)
    term.open(containerRef.current)
    fitAddon.fit()

    termRef.current = term
    fitRef.current = fitAddon
    searchRef.current = searchAddon

    return term
  }, [containerRef])

  const connectWebSocket = useCallback((assetId: number, termId: string) => {
    const token = localStorage.getItem('token')
    const ws = new WebSocket(
      `ws://${window.location.host}/api/ws/terminal?asset_id=${assetId}&token=${token}`
    )
    wsRef.current = ws

    ws.onopen = () => {
      reconnectAttempt.current = 0
      setConnected(termId, true)
      // Fit terminal after connection
      fitRef.current?.fit()
      if (termRef.current) {
        const { cols, rows } = termRef.current
        ws.send(JSON.stringify({ type: 'resize', data: JSON.stringify({ cols, rows }) }))
      }
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'output' && termRef.current) {
          termRef.current.write(msg.data)
          appendOutput(termId, msg.data)
        }
      } catch { /* ignore */ }
    }

    ws.onclose = () => {
      setConnected(termId, false)
      // Auto-reconnect with exponential backoff
      if (reconnectAttempt.current < maxReconnectAttempts) {
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempt.current), 30000)
        setTimeout(() => {
          reconnectAttempt.current++
          connectWebSocket(assetId, termId)
        }, delay)
      }
    }

    // Terminal input → WebSocket
    termRef.current?.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }))
      }
    })

    return ws
  }, [setConnected, appendOutput])

  const fit = useCallback(() => {
    fitRef.current?.fit()
    if (termRef.current && wsRef.current?.readyState === WebSocket.OPEN) {
      const { cols, rows } = termRef.current
      wsRef.current.send(JSON.stringify({ type: 'resize', data: JSON.stringify({ cols, rows }) }))
    }
  }, [])

  const search = useCallback((term: string) => {
    searchRef.current?.findNext(term)
  }, [])

  const dispose = useCallback(() => {
    wsRef.current?.close()
    termRef.current?.dispose()
    termRef.current = null
    fitRef.current = null
    searchRef.current = null
  }, [])

  // Resize observer
  useEffect(() => {
    if (!containerRef.current) return
    const observer = new ResizeObserver(() => fit())
    observer.observe(containerRef.current)
    return () => observer.disconnect()
  }, [containerRef, fit])

  return {
    terminal: termRef.current,
    initTerminal,
    connectWebSocket,
    fit,
    search,
    dispose,
  }
}
