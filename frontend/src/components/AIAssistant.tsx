import React, { useState, useEffect, useRef } from 'react'
import { Card, Input, Button, List, Select, Space, Typography, Modal, message, Tag, Popconfirm } from 'antd'
import { SendOutlined, PlusOutlined, DeleteOutlined, RobotOutlined, CheckCircleOutlined, CodeOutlined } from '@ant-design/icons'
import { aiApi } from '../services/api'
import { useTerminalStore } from '../stores/terminalStore'

const { Text } = Typography

/** Extract command blocks from AI response (```...``` or lines starting with $ or #) */
function extractCommands(content: string): string[] {
  const commands: string[] = []
  // Match fenced code blocks
  const codeBlockRegex = /```(?:bash|sh|shell|zsh)?\s*\n([\s\S]*?)```/g
  let match
  while ((match = codeBlockRegex.exec(content)) !== null) {
    const lines = match[1].trim().split('\n').filter(l => l.trim() && !l.trim().startsWith('#'))
    commands.push(...lines)
  }
  // Match lines starting with $ or > (prompt indicators)
  const promptRegex = /^[\$>]\s+(.+)$/gm
  while ((match = promptRegex.exec(content)) !== null) {
    if (!commands.includes(match[1].trim())) {
      commands.push(match[1].trim())
    }
  }
  return commands
}

const AIAssistant: React.FC = () => {
  const [sessions, setSessions] = useState<any[]>([])
  const [activeSession, setActiveSession] = useState<number | null>(null)
  const [messages, setMessages] = useState<any[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [model, setModel] = useState('gpt-4')
  const [confirmModal, setConfirmModal] = useState<{ open: boolean; commands: string[]; rawContent: string }>({ open: false, commands: [], rawContent: '' })
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const tabs = useTerminalStore((s) => s.tabs)
  const activeTabId = useTerminalStore((s) => s.activeTabId)

  useEffect(() => { aiApi.listSessions().then(r => setSessions(r.data || [])).catch(() => {}) }, [])

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages])

  const loadHistory = async (sessionId: number) => {
    setActiveSession(sessionId)
    const res = await aiApi.getHistory(sessionId)
    setMessages(res.data || [])
  }

  /** Send command to the active terminal via WebSocket */
  const sendCommandToTerminal = (command: string) => {
    const activeTab = tabs.find(t => t.id === activeTabId)
    if (!activeTab) {
      message.warning('没有活动的终端')
      return
    }
    // Dispatch a custom event that TerminalView listens for
    window.dispatchEvent(new CustomEvent('shelly:terminal-send', {
      detail: { tabId: activeTabId, data: command + '\n' },
    }))
    message.success(`已发送到终端: ${command.slice(0, 50)}...`)
  }

  const handleSend = async () => {
    if (!input.trim()) return
    setLoading(true)
    const userMsg = { role: 'user', content: input }
    setMessages(prev => [...prev, userMsg])
    setInput('')

    // Get terminal context
    const activeTab = tabs.find(t => t.id === activeTabId)
    const context = activeTab?.lastOutput?.slice(-2000) || ''

    try {
      const res = await aiApi.chat(activeSession || undefined, input, context, model)
      if (res.body) {
        const reader = res.body.getReader()
        let assistantMsg = ''
        setMessages(prev => [...prev, { role: 'assistant', content: '' }])

        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          const text = new TextDecoder().decode(value)
          const lines = text.split('\n')
          for (const line of lines) {
            if (line.startsWith('data: ')) {
              try {
                const data = JSON.parse(line.slice(6))
                if (data.content) {
                  assistantMsg += data.content
                  setMessages(prev => [...prev.slice(0, -1), { role: 'assistant', content: assistantMsg }])
                }
              } catch { /* ignore */ }
            }
          }
        }

        // After streaming complete, check for commands that need confirmation
        const cmds = extractCommands(assistantMsg)
        if (cmds.length > 0) {
          setConfirmModal({ open: true, commands: cmds, rawContent: assistantMsg })
        }
      }
    } catch (err: any) {
      message.error(err.message)
    }
    setLoading(false)
  }

  const handleNewSession = async () => {
    setActiveSession(null)
    setMessages([])
  }

  /** Render a message with command blocks highlighted and confirm buttons */
  const renderMessage = (msg: any, i: number) => {
    const content = msg.content || ''
    // Split content by code blocks for rendering
    const parts: React.ReactNode[] = []
    let lastIndex = 0
    const codeBlockRegex = /```(?:bash|sh|shell|zsh)?\s*\n([\s\S]*?)```/g
    let match
    let key = 0

    while ((match = codeBlockRegex.exec(content)) !== null) {
      // Text before code block
      if (match.index > lastIndex) {
        parts.push(<span key={key++}>{content.slice(lastIndex, match.index)}</span>)
      }
      // Code block with confirm button
      const code = match[1].trim()
      parts.push(
        <div key={key++} style={{ background: '#1a1a2e', borderRadius: 6, padding: '8px 12px', margin: '4px 0', position: 'relative' }}>
          <pre style={{ margin: 0, color: '#a0d0a0', fontSize: 13, whiteSpace: 'pre-wrap' }}><code>{code}</code></pre>
          {msg.role === 'assistant' && (
            <Button size="small" type="primary" icon={<CheckCircleOutlined />}
              style={{ position: 'absolute', top: 4, right: 4 }}
              onClick={() => {
                const lines = code.split('\n').filter(l => l.trim() && !l.trim().startsWith('#'))
                lines.forEach(line => sendCommandToTerminal(line))
              }}>
              确认执行
            </Button>
          )}
        </div>
      )
      lastIndex = match.index + match[0].length
    }

    // Remaining text
    if (lastIndex < content.length) {
      parts.push(<span key={key++}>{content.slice(lastIndex)}</span>)
    }

    return parts.length > 0 ? parts : content
  }

  return (
    <div style={{ display: 'flex', height: '100%', gap: 16, padding: 16 }}>
      <Card style={{ width: 250, overflow: 'auto' }} title={<><RobotOutlined /> 对话</>} size="small"
        extra={<Button size="small" icon={<PlusOutlined />} onClick={handleNewSession}>新建</Button>}>
        <List dataSource={sessions} size="small" renderItem={(s: any) => (
          <List.Item onClick={() => loadHistory(s.id)} style={{ cursor: 'pointer', background: s.id === activeSession ? '#2d4a7c33' : undefined, padding: '4px 8px', borderRadius: 4 }}>
            <Text ellipsis style={{ fontSize: 12 }}>{s.title || '新对话'}</Text>
          </List.Item>
        )} />
      </Card>
      <Card style={{ flex: 1, display: 'flex', flexDirection: 'column' }} bodyStyle={{ flex: 1, display: 'flex', flexDirection: 'column', padding: 0 }}>
        <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
          {messages.map((msg, i) => (
            <div key={i} style={{ marginBottom: 12, textAlign: msg.role === 'user' ? 'right' : 'left' }}>
              <Tag color={msg.role === 'user' ? 'blue' : 'default'}>{msg.role === 'user' ? '你' : 'AI'}</Tag>
              <div style={{ display: 'inline-block', padding: '8px 12px', borderRadius: 8, background: msg.role === 'user' ? '#2d4a7c' : '#333', maxWidth: '80%', textAlign: 'left', whiteSpace: 'pre-wrap' }}>
                {renderMessage(msg, i)}
              </div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>
        <div style={{ padding: 12, borderTop: '1px solid #333', display: 'flex', gap: 8 }}>
          <Select value={model} onChange={setModel} style={{ width: 120 }} size="small"
            options={[{ value: 'gpt-4', label: 'GPT-4' }, { value: 'gpt-3.5-turbo', label: 'GPT-3.5' }, { value: 'claude-3', label: 'Claude 3' }]} />
          <Input value={input} onChange={e => setInput(e.target.value)} onPressEnter={handleSend}
            placeholder="输入消息..." disabled={loading} suffix={<Button type="primary" size="small" icon={<SendOutlined />} loading={loading} onClick={handleSend} />} />
        </div>
      </Card>

      {/* Command Confirmation Modal */}
      <Modal
        title={<><CodeOutlined /> 命令确认 - AI 生成的命令需要确认后才执行</>}
        open={confirmModal.open}
        onCancel={() => setConfirmModal({ open: false, commands: [], rawContent: '' })}
        footer={[
          <Button key="cancel" onClick={() => setConfirmModal({ open: false, commands: [], rawContent: '' })}>取消</Button>,
          <Button key="all" type="primary" onClick={() => {
            confirmModal.commands.forEach(cmd => sendCommandToTerminal(cmd))
            setConfirmModal({ open: false, commands: [], rawContent: '' })
          }}>全部执行</Button>,
        ]}
      >
        <div style={{ marginBottom: 12, color: '#999' }}>AI 检测到以下命令，请逐条确认或全部执行：</div>
        {confirmModal.commands.map((cmd, i) => (
          <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0', borderBottom: '1px solid #333' }}>
            <code style={{ flex: 1, color: '#a0d0a0', fontSize: 13 }}>{cmd}</code>
            <Popconfirm title="确认执行此命令?" onConfirm={() => sendCommandToTerminal(cmd)}>
              <Button size="small" type="primary" icon={<CheckCircleOutlined />}>执行</Button>
            </Popconfirm>
          </div>
        ))}
      </Modal>
    </div>
  )
}

export default AIAssistant
