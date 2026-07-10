import React, { useState, useEffect, useRef } from 'react'
import { Card, Select, Button, Input, Space, Table, Tag, message, Divider, Statistic, Row, Col } from 'antd'
import { PlayCircleOutlined, StopOutlined, ClearOutlined } from '@ant-design/icons'
import { assetApi } from '../services/api'

interface AssetResult {
  asset_id: number
  name: string
  output: string
  status: 'pending' | 'connecting' | 'connected' | 'done' | 'error'
  error?: string
}

const BatchExec: React.FC = () => {
  const [assets, setAssets] = useState<any[]>([])
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [command, setCommand] = useState('')
  const [running, setRunning] = useState(false)
  const [results, setResults] = useState<Map<number, AssetResult>>(new Map())
  const [stats, setStats] = useState({ total: 0, done: 0, errors: 0 })
  const wsRef = useRef<WebSocket | null>(null)
  const outputRefs = useRef<Map<number, HTMLPreElement>>(new Map())

  useEffect(() => { assetApi.list().then(r => setAssets(r.data || [])).catch(() => {}) }, [])

  const handleStart = () => {
    if (!selectedIds.length || !command.trim()) {
      message.warning('请选择资产并输入命令')
      return
    }

    setRunning(true)
    const resultMap = new Map<number, AssetResult>()
    selectedIds.forEach(id => {
      const asset = assets.find(a => a.id === id)
      resultMap.set(id, { asset_id: id, name: asset?.name || '', output: '', status: 'pending' })
    })
    setResults(resultMap)
    setStats({ total: selectedIds.length, done: 0, errors: 0 })

    const token = localStorage.getItem('token')
    const ws = new WebSocket(`ws://${window.location.host}/api/ws/batch-exec?token=${token}`)
    wsRef.current = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ asset_ids: selectedIds, command }))
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        setResults(prev => {
          const next = new Map(prev)
          const id = msg.asset_id
          const existing = next.get(id)

          switch (msg.type) {
            case 'connecting':
              if (existing) next.set(id, { ...existing, status: 'connecting' })
              break
            case 'connected':
              if (existing) next.set(id, { ...existing, status: 'connected' })
              break
            case 'output':
              if (existing) {
                next.set(id, { ...existing, output: existing.output + msg.data, status: 'connected' })
                // Auto-scroll output
                setTimeout(() => {
                  const el = outputRefs.current.get(id)
                  if (el) el.scrollTop = el.scrollHeight
                }, 0)
              }
              break
            case 'done':
              if (existing) next.set(id, { ...existing, status: 'done' })
              setStats(s => ({ ...s, done: s.done + 1 }))
              break
            case 'error':
              if (msg.asset_id) {
                if (existing) next.set(id, { ...existing, status: 'error', error: msg.error })
                setStats(s => ({ ...s, done: s.done + 1, errors: s.errors + 1 }))
              }
              break
          }
          return next
        })
      } catch { /* ignore parse errors */ }
    }

    ws.onclose = () => {
      setRunning(false)
      setStats(s => ({ ...s, done: selectedIds.length }))
    }

    ws.onerror = () => {
      message.error('WebSocket 连接失败')
      setRunning(false)
    }
  }

  const handleStop = () => {
    wsRef.current?.close()
    setRunning(false)
  }

  const handleClear = () => {
    setResults(new Map())
    setStats({ total: 0, done: 0, errors: 0 })
  }

  const statusColor = (status: string) => {
    switch (status) {
      case 'pending': return 'default'
      case 'connecting': return 'processing'
      case 'connected': return 'blue'
      case 'done': return 'success'
      case 'error': return 'error'
      default: return 'default'
    }
  }

  const resultArray = Array.from(results.values())

  return (
    <div style={{ padding: 24 }}>
      <Card style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Card title="选择目标资产" size="small">
              <Select mode="multiple" style={{ width: '100%' }} placeholder="选择资产"
                value={selectedIds} onChange={setSelectedIds}
                options={assets.map(a => ({ value: a.id, label: `${a.name} (${a.host})` }))} />
            </Card>
          </Col>
          <Col span={12}>
            <Card title="执行命令" size="small">
              <Space.Compact style={{ width: '100%' }}>
                <Input value={command} onChange={e => setCommand(e.target.value)}
                  onPressEnter={handleStart} disabled={running} placeholder="输入要执行的命令..." />
                {running ? (
                  <Button danger icon={<StopOutlined />} onClick={handleStop}>停止</Button>
                ) : (
                  <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleStart}
                    disabled={!selectedIds.length || !command.trim()}>执行</Button>
                )}
                <Button icon={<ClearOutlined />} onClick={handleClear}>清空</Button>
              </Space.Compact>
            </Card>
          </Col>
          <Col span={4}>
            <Row gutter={8}>
              <Col span={8}><Statistic title="总数" value={stats.total} /></Col>
              <Col span={8}><Statistic title="完成" value={stats.done} valueStyle={{ color: '#3f8600' }} /></Col>
              <Col span={8}><Statistic title="失败" value={stats.errors} valueStyle={{ color: stats.errors > 0 ? '#cf1322' : undefined }} /></Col>
            </Row>
          </Col>
        </Row>
      </Card>

      {resultArray.length > 0 && (
        <div style={{ display: 'grid', gridTemplateColumns: resultArray.length <= 2 ? `repeat(${resultArray.length}, 1fr)` : 'repeat(2, 1fr)', gap: 12 }}>
          {resultArray.map(r => (
            <Card key={r.asset_id} size="small"
              title={<Space>{r.name} <Tag color={statusColor(r.status)}>{r.status}</Tag></Space>}
              bodyStyle={{ padding: 0, maxHeight: 400, overflow: 'auto' }}>
              <pre ref={el => { if (el) outputRefs.current.set(r.asset_id, el) }}
                style={{ margin: 0, padding: 12, fontSize: 12, fontFamily: 'monospace', whiteSpace: 'pre-wrap', background: '#1e1e1e', color: '#d4d4d4', minHeight: 100 }}>
                {r.output || (r.status === 'pending' ? '等待连接...' : r.status === 'connecting' ? '正在连接...' : '')}
                {r.error && <span style={{ color: '#f5222d' }}>{r.error}</span>}
              </pre>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}

export default BatchExec
