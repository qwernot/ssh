import React, { useState, useEffect, useRef, useCallback } from 'react'
import { Card, Table, Button, Space, Modal, Slider, Select, Tag, message, Popconfirm, Empty } from 'antd'
import { PlayCircleOutlined, PauseCircleOutlined, DownloadOutlined, DeleteOutlined } from '@ant-design/icons'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

interface SessionRecord {
  id: number
  asset_name: string
  title: string
  duration: number
  file_size: number
  created_at: string
  file_path: string
}

interface CastEvent {
  time: number
  type: string
  data?: string
  [key: string]: any
}

interface CastRecording {
  version: number
  width: number
  height: number
  title: string
  duration?: number
  events: CastEvent[]
}

const SessionPlayer: React.FC = () => {
  const [sessions, setSessions] = useState<SessionRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [playerOpen, setPlayerOpen] = useState(false)
  const [currentSession, setCurrentSession] = useState<SessionRecord | null>(null)
  const [recording, setRecording] = useState<CastRecording | null>(null)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [speed, setSpeed] = useState(1)
  const timerRef = useRef<number | null>(null)
  const eventIndexRef = useRef(0)
  const startTimeRef = useRef(0)
  const pausedAtRef = useRef(0)
  const termRef = useRef<Terminal | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const speedRef = useRef(1)

  const loadSessions = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/sessions', { headers: { Authorization: `Bearer ${localStorage.getItem('token')}` } })
      const data = await res.json()
      setSessions(data.data || [])
    } catch { /* ignore */ }
    setLoading(false)
  }, [])

  useEffect(() => { loadSessions() }, [loadSessions])

  const handlePlay = async (session: SessionRecord) => {
    setCurrentSession(session)
    setPlayerOpen(true)
    setPlaying(false)
    setCurrentTime(0)
    eventIndexRef.current = 0
    pausedAtRef.current = 0

    try {
      const res = await fetch(`/api/sessions/${session.id}/record`, {
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
      })
      const data: CastRecording = await res.json()
      setRecording(data)
    } catch (err: any) {
      message.error('加载录像失败')
    }
  }

  const startPlayback = useCallback((fromTime: number) => {
    if (!recording || !recording.events.length || !termRef.current) return
    const term = termRef.current

    setPlaying(true)
    startTimeRef.current = Date.now() - fromTime * 1000
    pausedAtRef.current = fromTime
    speedRef.current = speed

    // Find starting event index
    let startIdx = 0
    for (let i = 0; i < recording.events.length; i++) {
      if (recording.events[i].time <= fromTime) {
        startIdx = i
      } else break
    }
    eventIndexRef.current = startIdx

    // Rebuild terminal content up to fromTime
    term.clear()
    for (let i = 0; i < startIdx; i++) {
      const ev = recording.events[i]
      if (ev.type === 'o' && ev.data) term.write(ev.data)
    }

    const tick = () => {
      const elapsed = (Date.now() - startTimeRef.current) / 1000 * speedRef.current
      const duration = recording.duration || (recording.events.length > 0 ? recording.events[recording.events.length - 1].time : 0)

      while (eventIndexRef.current < recording.events.length && recording.events[eventIndexRef.current].time <= elapsed) {
        const ev = recording.events[eventIndexRef.current]
        if (ev.type === 'o' && ev.data) {
          term.write(ev.data)
        } else if (ev.type === 'r' && ev.width && ev.height) {
          term.resize(ev.width, ev.height)
        }
        eventIndexRef.current++
      }

      setCurrentTime(Math.min(elapsed, duration))

      if (elapsed >= duration) {
        setPlaying(false)
        return
      }

      timerRef.current = requestAnimationFrame(tick)
    }

    timerRef.current = requestAnimationFrame(tick)
  }, [recording, speed])

  const handleTogglePlay = () => {
    if (playing) {
      // Pause
      if (timerRef.current) cancelAnimationFrame(timerRef.current)
      pausedAtRef.current = currentTime
      setPlaying(false)
    } else {
      // Resume
      startPlayback(pausedAtRef.current)
    }
  }

  const handleSeek = (value: number) => {
    if (timerRef.current) cancelAnimationFrame(timerRef.current)
    setPlaying(false)
    pausedAtRef.current = value
    setCurrentTime(value)

    // Rebuild terminal content up to seek position
    if (recording && termRef.current) {
      termRef.current.clear()
      let lastIdx = recording.events.length
      for (let i = 0; i < recording.events.length; i++) {
        if (recording.events[i].time > value) { lastIdx = i; break }
        if (recording.events[i].type === 'o' && recording.events[i].data) {
          termRef.current.write(recording.events[i].data!)
        }
      }
      eventIndexRef.current = lastIdx
    }
  }

  const handleSpeedChange = (newSpeed: number) => {
    setSpeed(newSpeed)
    speedRef.current = newSpeed
    if (playing) {
      if (timerRef.current) cancelAnimationFrame(timerRef.current)
      startPlayback(currentTime)
    }
  }

  const handleDownload = async (session: SessionRecord) => {
    const token = localStorage.getItem('token')
    const a = document.createElement('a')
    a.href = `/api/sessions/${session.id}/download?token=${token}`
    a.download = `${session.title}.cast`
    a.click()
  }

  const handleDelete = async (id: number) => {
    await fetch(`/api/sessions/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    })
    message.success('已删除')
    loadSessions()
  }

  useEffect(() => {
    return () => { if (timerRef.current) cancelAnimationFrame(timerRef.current) }
  }, [])

  // Initialize xterm.js Terminal for playback
  useEffect(() => {
    if (!playerOpen || !containerRef.current) return

    const term = new Terminal({
      cursorBlink: false,
      fontSize: 13,
      fontFamily: "'Cascadia Code', 'Fira Code', Monaco, Menlo, monospace",
      theme: { background: '#1e1e1e', foreground: '#d4d4d4' },
      cols: recording?.width || 80,
      rows: recording?.height || 24,
      allowProposedApi: true,
    })
    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(containerRef.current)
    fitAddon.fit()
    termRef.current = term
    fitRef.current = fitAddon

    return () => {
      term.dispose()
      termRef.current = null
    }
  }, [playerOpen, recording])

  const duration = recording?.duration || 0
  const columns = [
    { title: '标题', dataIndex: 'title', key: 'title' },
    { title: '资产', dataIndex: 'asset_name', key: 'asset_name' },
    { title: '时长', dataIndex: 'duration', key: 'duration', render: (d: number) => `${Math.floor(d / 60)}:${(d % 60).toString().padStart(2, '0')}` },
    { title: '大小', dataIndex: 'file_size', key: 'file_size', render: (s: number) => s > 1024 ? `${(s / 1024).toFixed(1)}KB` : `${s}B` },
    { title: '时间', dataIndex: 'created_at', key: 'created_at', render: (t: string) => new Date(t).toLocaleString() },
    {
      title: '操作', key: 'action', width: 200,
      render: (_: any, r: SessionRecord) => (
        <Space>
          <Button size="small" icon={<PlayCircleOutlined />} onClick={() => handlePlay(r)}>回放</Button>
          <Button size="small" icon={<DownloadOutlined />} onClick={() => handleDownload(r)} />
          <Popconfirm title="确定删除?" onConfirm={() => handleDelete(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title="会话录制">
        <Table columns={columns} dataSource={sessions} rowKey="id" loading={loading} size="small" />
      </Card>

      <Modal title={currentSession?.title || '会话回放'} open={playerOpen} onCancel={() => { setPlayerOpen(false); if (timerRef.current) cancelAnimationFrame(timerRef.current); setPlaying(false) }}
        width={900} footer={null} destroyOnClose>
        {recording ? (
          <div>
            {/* Terminal playback */}
            <div ref={containerRef} style={{ height: 400, borderRadius: 8, marginBottom: 16, overflow: 'hidden' }} />

            {/* Controls */}
            <Space style={{ width: '100%' }} direction="vertical">
              <Slider min={0} max={duration} step={0.1} value={currentTime}
                onChange={handleSeek} tooltip={{ formatter: (v) => `${Math.floor((v || 0) / 60)}:${((v || 0) % 60).toFixed(0).padStart(2, '0')}` }} />
              <Space>
                <Button icon={playing ? <PauseCircleOutlined /> : <PlayCircleOutlined />} onClick={handleTogglePlay} type="primary">
                  {playing ? '暂停' : '播放'}
                </Button>
                <Select value={speed} onChange={handleSpeedChange} style={{ width: 100 }}
                  options={[
                    { value: 0.5, label: '0.5x' },
                    { value: 1, label: '1x' },
                    { value: 2, label: '2x' },
                    { value: 4, label: '4x' },
                    { value: 8, label: '8x' },
                  ]} />
                <Tag>{Math.floor(currentTime / 60)}:{Math.floor(currentTime % 60).toString().padStart(2, '0')} / {Math.floor(duration / 60)}:{Math.floor(duration % 60).toString().padStart(2, '0')}</Tag>
              </Space>
            </Space>
          </div>
        ) : (
          <Empty description="加载中..." />
        )}
      </Modal>
    </div>
  )
}

export default SessionPlayer
