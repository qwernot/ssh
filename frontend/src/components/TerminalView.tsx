import React, { useEffect, useRef, useState, useCallback } from 'react'
import { Tabs, Button, Space, Dropdown, Input, Tooltip, Modal, List, Tag, Progress, message } from 'antd'
import { CloseOutlined, SearchOutlined, ColumnWidthOutlined, ExpandOutlined, SendOutlined, DisconnectOutlined, FileZipOutlined } from '@ant-design/icons'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { WebglAddon } from '@xterm/addon-webgl'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import '@xterm/xterm/css/xterm.css'
import { useTerminalStore } from '../stores/terminalStore'
import { snippetApi, highlightApi } from '../services/api'
import { useZmodem } from '../hooks/useZmodem'

declare var TextDecoder: any

const TerminalView: React.FC = () => {
  const { tabs, activeTabId, setActiveTab, removeTab } = useTerminalStore()
  const [showSearch, setShowSearch] = useState(false)
  const [searchText, setSearchText] = useState('')
  const [showSnippets, setShowSnippets] = useState(false)
  const [snippets, setSnippets] = useState<any[]>([])
  const [highlights, setHighlights] = useState<any[]>([])
  const [splitDir, setSplitDir] = useState<'none' | 'h' | 'v'>('none')

  useEffect(() => {
    snippetApi.list().then(r => setSnippets(r.data || [])).catch(() => {})
    highlightApi.list().then(r => setHighlights(r.data || [])).catch(() => {})
  }, [])

  if (tabs.length === 0) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: '#666' }}>
        <p>请从资产管理连接终端，或点击"连接"按钮</p>
      </div>
    )
  }

  const tabItems = tabs.map(tab => ({
    key: tab.id,
    label: (
      <span>
        {tab.asset.name}
        {!tab.connected && <Tag color="red" style={{ marginLeft: 4, fontSize: 10 }}>断开</Tag>}
      </span>
    ),
    children: <TerminalInstance tabId={tab.id} asset={tab.asset} highlights={highlights} />,
  }))

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 8px', background: '#1a1a1a', borderBottom: '1px solid #333' }}>
        <Space>
          <Tooltip title="搜索"><Button size="small" icon={<SearchOutlined />} onClick={() => setShowSearch(!showSearch)} /></Tooltip>
          <Tooltip title="命令片段"><Button size="small" icon={<SendOutlined />} onClick={() => setShowSnippets(true)} /></Tooltip>
          <Tooltip title="水平分屏"><Button size="small" icon={<ColumnWidthOutlined />} onClick={() => setSplitDir(splitDir === 'h' ? 'none' : 'h')} /></Tooltip>
          <Tooltip title="垂直分屏"><Button size="small" icon={<ExpandOutlined />} onClick={() => setSplitDir(splitDir === 'v' ? 'none' : 'v')} /></Tooltip>
        </Space>
      </div>
      {showSearch && (
        <div style={{ padding: '4px 8px', background: '#1a1a1a' }}>
          <Input.Search placeholder="搜索终端内容..." value={searchText} onChange={e => setSearchText(e.target.value)} size="small" style={{ width: 300 }} />
        </div>
      )}
      <div style={{ flex: 1, display: splitDir === 'none' ? 'block' : 'flex', flexDirection: splitDir === 'v' ? 'column' : 'row' }}>
        <div style={{ flex: 1, overflow: 'hidden' }}>
          <Tabs activeKey={activeTabId || undefined} onChange={setActiveTab} type="editable-card" hideAdd
            onEdit={(targetKey, action) => { if (action === 'remove') removeTab(targetKey as string) }}
            items={tabItems} size="small"
          />
        </div>
      </div>
      <Modal title="命令片段" open={showSnippets} onCancel={() => setShowSnippets(false)} footer={null}>
        <List dataSource={snippets} renderItem={(item: any) => (
          <List.Item actions={[<Button size="small" type="primary" onClick={() => {
            if (!activeTabId) { message.warning('没有活动的终端'); return }
            window.dispatchEvent(new CustomEvent('shelly:terminal-send', {
              detail: { tabId: activeTabId, data: item.command + '\n' },
            }))
            message.success(`已发送: ${item.title}`)
            setShowSnippets(false)
          }}>发送</Button>]}>
            <List.Item.Meta title={item.title} description={<code>{item.command}</code>} />
          </List.Item>
        )} />
      </Modal>
    </div>
  )
}

interface TerminalInstanceProps {
  tabId: string
  asset: any
  highlights: any[]
}

const TerminalInstance: React.FC<TerminalInstanceProps> = ({ tabId, asset, highlights }) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const searchRef = useRef<SearchAddon | null>(null)
  const reconnectTimer = useRef<any>(null)
  const { setConnected, appendOutput } = useTerminalStore()
  const zmodem = useZmodem()
  const decoderRef = useRef<any>(null)
  const decorationRefs = useRef<any[]>([])

  const connect = useCallback(() => {
    const token = localStorage.getItem('shelly_token') || ''
    const wsUrl = `ws://${window.location.host}/api/ws/terminal?asset_id=${asset.id}&token=${token}`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(tabId, true)
      if (reconnectTimer.current) { clearTimeout(reconnectTimer.current); reconnectTimer.current = null }
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'output' && msg.data) {
          let decoded: string

          // GBK / other encoding support via TextDecoder
          if (asset.encoding && asset.encoding.toLowerCase() !== 'utf-8' && decoderRef.current) {
            try {
              const binaryStr = atob(msg.data)
              const bytes = new Uint8Array(binaryStr.length)
              for (let i = 0; i < binaryStr.length; i++) bytes[i] = binaryStr.charCodeAt(i)
              decoded = decoderRef.current.decode(bytes)
            } catch {
              decoded = msg.data.replace(/\\n/g, '\n').replace(/\\r/g, '\r').replace(/\\t/g, '\t')
            }
          } else {
            decoded = msg.data.replace(/\\n/g, '\n').replace(/\\r/g, '\r').replace(/\\t/g, '\t')
          }

          // ZModem detection
          if (zmodem.detectZmodem(decoded)) {
            zmodem.startReceive('zmodem_transfer')
            message.info('ZModem 传输检测到，正在接收...')
          }

          termRef.current?.write(decoded)
          appendOutput(tabId, decoded)
        } else if (msg.type === 'close') {
          setConnected(tabId, false)
        }
      } catch { /* ignore parse errors */ }
    }

    ws.onclose = () => {
      setConnected(tabId, false)
      // Auto-reconnect with exponential backoff
      if (!reconnectTimer.current) {
        let delay = 1000
        const tryReconnect = () => {
          reconnectTimer.current = setTimeout(() => {
            reconnectTimer.current = null
            connect()
            delay = Math.min(delay * 2, 30000)
          }, delay)
        }
        tryReconnect()
      }
    }

    ws.onerror = () => { ws.close() }
  }, [tabId, asset.id])

  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: "'Cascadia Code', 'Fira Code', Monaco, Menlo, monospace",
      theme: { background: '#1a1a1a', foreground: '#e0e0e0', cursor: '#ffffff', selectionBackground: '#2d4a7c' },
      allowProposedApi: true,
    })

    const fitAddon = new FitAddon()
    const searchAddon = new SearchAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(searchAddon)
    term.loadAddon(new WebLinksAddon())
    const unicode11Addon = new Unicode11Addon()
    term.loadAddon(unicode11Addon)
    unicode11Addon.activate(term)
    term.open(containerRef.current)

    try { term.loadAddon(new WebglAddon()) } catch { /* fallback to canvas */ }

    fitAddon.fit()
    termRef.current = term
    fitRef.current = fitAddon
    searchRef.current = searchAddon

    // Apply keyword highlighting using xterm decoration API
    const applyHighlights = () => {
      decorationRefs.current.forEach(d => d.dispose())
      decorationRefs.current = []
      const buffer = term.buffer.active
      highlights.forEach((rule: any) => {
        if (!rule.enabled || !rule.keyword || !rule.color) return
        const keyword = rule.keyword.toLowerCase()
        for (let i = 0; i < buffer.length; i++) {
          const line = buffer.getLine(i)
          if (!line) continue
          const text = line.translateToString(true).toLowerCase()
          if (text.includes(keyword)) {
            const marker = term.registerMarker(i - buffer.baseY)
            if (marker) {
              const decoration = term.registerDecoration({
                marker,
                backgroundColor: rule.color + '33',
                foregroundColor: rule.color,
              })
              if (decoration) decorationRefs.current.push(decoration)
            }
          }
        }
      })
    }
    applyHighlights()

    // Initialize encoding decoder for GBK etc.
    if (asset.encoding && asset.encoding.toLowerCase() !== 'utf-8') {
      try { decoderRef.current = new (globalThis as any).TextDecoder(asset.encoding) } catch { decoderRef.current = null }
    } else {
      decoderRef.current = null
    }

    connect()

    term.onData((data) => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: 'input', data }))
      }
    })

    term.onResize(({ cols, rows }) => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: 'resize', cols, rows }))
      }
    })

    const resizeObserver = new ResizeObserver(() => { fitAddon.fit() })
    resizeObserver.observe(containerRef.current)

    // Listen for commands sent from AI Assistant or Snippets
    const handleTerminalSend = (e: Event) => {
      const detail = (e as CustomEvent).detail
      if (detail.tabId === tabId && wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: 'input', data: detail.data }))
      }
    }
    window.addEventListener('shelly:terminal-send', handleTerminalSend)

    return () => {
      wsRef.current?.close()
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      window.removeEventListener('shelly:terminal-send', handleTerminalSend)
      decorationRefs.current.forEach(d => d.dispose())
      term.dispose()
      resizeObserver.disconnect()
    }
  }, [])

  return (
    <div style={{ position: 'relative' }}>
      <div ref={containerRef} style={{ width: '100%', height: 'calc(100vh - 120px)' }} />
      {zmodem.transferring && (
        <div style={{ position: 'absolute', bottom: 16, left: '50%', transform: 'translateX(-50%)', background: 'rgba(0,0,0,0.85)', borderRadius: 8, padding: '12px 20px', minWidth: 300, zIndex: 10 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
            <FileZipOutlined style={{ color: '#1890ff' }} />
            <span style={{ color: '#fff', fontSize: 13 }}>{zmodem.transferName || 'ZModem 传输中...'}</span>
            <Button size="small" type="text" onClick={zmodem.cancel} style={{ color: '#ff4d4f', marginLeft: 'auto' }}>取消</Button>
          </div>
          <Progress percent={zmodem.transferProgress} size="small" strokeColor="#1890ff" />
        </div>
      )}
    </div>
  )
}

export default TerminalView
