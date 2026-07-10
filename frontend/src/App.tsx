import React, { useState, useEffect, useRef, useCallback } from 'react'
import { Input, Button, Card, Typography, message } from 'antd'
import { LockOutlined } from '@ant-design/icons'
import { useAuthStore } from './stores/authStore'
import Login from './components/Login'
import MainLayout from './components/MainLayout'

const { Title, Text } = Typography

const App: React.FC = () => {
  const token = useAuthStore((s) => s.token)
  const [locked, setLocked] = useState(false)
  const [lockPin, setLockPin] = useState('')
  const [lockEnabled, setLockEnabled] = useState(false)
  const [lockTimeout, setLockTimeout] = useState(300) // default 5 min
  const lastActivityRef = useRef(Date.now())
  const timerRef = useRef<number | null>(null)

  // Check app lock status on mount
  useEffect(() => {
    if (!token) return
    fetch('/api/applock/status', { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.json())
      .then(r => {
        setLockEnabled(r.enabled)
        if (r.enabled) setLocked(true)
      })
      .catch(() => {})

    // Get lock timeout from settings
    fetch('/api/settings', { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.json())
      .then(r => {
        if (r.data?.lock_timeout) setLockTimeout(r.data.lock_timeout)
      })
      .catch(() => {})
  }, [token])

  // Auto-lock on inactivity timeout
  const checkInactivity = useCallback(() => {
    if (!lockEnabled || !locked) return
    const elapsed = (Date.now() - lastActivityRef.current) / 1000
    if (elapsed >= lockTimeout) {
      setLocked(true)
    }
  }, [lockEnabled, locked, lockTimeout])

  useEffect(() => {
    if (!lockEnabled || !token) return

    const handleActivity = () => { lastActivityRef.current = Date.now() }
    const events = ['mousedown', 'mousemove', 'keydown', 'scroll', 'touchstart']
    events.forEach(e => window.addEventListener(e, handleActivity))

    timerRef.current = window.setInterval(checkInactivity, 10000)

    return () => {
      events.forEach(e => window.removeEventListener(e, handleActivity))
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [lockEnabled, token, checkInactivity])

  const handleUnlock = async () => {
    const res = await fetch('/api/applock/unlock', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ pin: lockPin })
    })
    if (res.ok) {
      setLocked(false)
      setLockPin('')
      lastActivityRef.current = Date.now()
      message.success('已解锁')
    } else {
      message.error('PIN 错误')
    }
  }

  if (!token) {
    return <Login />
  }

  // Show lock screen
  if (locked && lockEnabled) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh', background: '#141414' }}>
        <Card style={{ width: 360, textAlign: 'center' }}>
          <LockOutlined style={{ fontSize: 48, color: '#1890ff', marginBottom: 16 }} />
          <Title level={4}>应用已锁定</Title>
          <Text type="secondary">请输入 PIN 码解锁</Text>
          <div style={{ marginTop: 24 }}>
            <Input.Password
              value={lockPin}
              onChange={e => setLockPin(e.target.value)}
              onPressEnter={handleUnlock}
              placeholder="输入 PIN 码"
              size="large"
              style={{ marginBottom: 16 }}
            />
            <Button type="primary" block size="large" onClick={handleUnlock}>解锁</Button>
          </div>
        </Card>
      </div>
    )
  }

  return <MainLayout />
}

export default App
