import React, { useState, useEffect } from 'react'
import { Card, Form, Input, Select, Switch, Button, message, Space, Table, Popconfirm, Tag } from 'antd'
import { SaveOutlined, SyncOutlined, KeyOutlined, DeleteOutlined, PlusOutlined, LockOutlined } from '@ant-design/icons'
import { settingsApi, syncApi, tokenApi, authApi } from '../services/api'

const AppLockSection: React.FC = () => {
  const [lockEnabled, setLockEnabled] = useState(false)
  const [pinForm] = Form.useForm()
  const [disableForm] = Form.useForm()
  const token = localStorage.getItem('token') || ''

  useEffect(() => {
    fetch('/api/applock/status', { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.json()).then(r => setLockEnabled(r.enabled)).catch(() => {})
  }, [token])

  const handleSetLock = async (values: any) => {
    const res = await fetch('/api/applock/set', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ pin: values.pin })
    })
    if (res.ok) { message.success('应用锁已启用'); setLockEnabled(true); pinForm.resetFields() }
    else { const d = await res.json(); message.error(d.error) }
  }

  const handleDisableLock = async (values: any) => {
    const res = await fetch('/api/applock/disable', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ pin: values.disablePin })
    })
    if (res.ok) { message.success('应用锁已禁用'); setLockEnabled(false); disableForm.resetFields() }
    else { const d = await res.json(); message.error(d.error) }
  }

  return (
    <div>
      {lockEnabled ? (
        <Form form={disableForm} layout="inline" onFinish={handleDisableLock}>
          <Form.Item name="disablePin" label="输入PIN解除" rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item>
            <Button type="primary" danger htmlType="submit">禁用应用锁</Button>
          </Form.Item>
        </Form>
      ) : (
        <Form form={pinForm} layout="inline" onFinish={handleSetLock}>
          <Form.Item name="pin" label="设置PIN码" rules={[{ required: true, min: 4 }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<LockOutlined />}>启用应用锁</Button>
          </Form.Item>
        </Form>
      )}
    </div>
  )
}

const Settings: React.FC = () => {
  const [settingsForm] = Form.useForm()
  const [syncForm] = Form.useForm()
  const [tokens, setTokens] = useState<any[]>([])

  useEffect(() => {
    settingsApi.get().then(r => settingsForm.setFieldsValue(r.data || {})).catch(() => {})
    syncApi.getConfig().then(r => syncForm.setFieldsValue(r.data || {})).catch(() => {})
    tokenApi.list().then(r => setTokens(r.data || [])).catch(() => {})
  }, [])

  const handleSaveSettings = async (values: any) => {
    await settingsApi.update(values)
    message.success('设置已保存')
  }

  const handleSaveSync = async (values: any) => {
    await syncApi.updateConfig(values)
    message.success('同步配置已保存')
  }

  const handleTriggerSync = async () => {
    await syncApi.trigger()
    message.success('同步已触发')
  }

  const handleCreateToken = async () => {
    const name = `token-${Date.now()}`
    await tokenApi.create(name)
    message.success('Token 已创建')
    tokenApi.list().then(r => setTokens(r.data || []))
  }

  const handleDeleteToken = async (id: number) => {
    await tokenApi.delete(id)
    message.success('Token 已删除')
    tokenApi.list().then(r => setTokens(r.data || []))
  }

  const handleChangePassword = async (values: any) => {
    try {
      await authApi.changePassword(values.oldPassword, values.newPassword)
      message.success('密码已修改')
    } catch (err: any) { message.error(err.message) }
  }

  return (
    <div style={{ padding: 24, maxWidth: 800 }}>
      <Card title="应用设置" style={{ marginBottom: 16 }}>
        <Form form={settingsForm} layout="vertical" onFinish={handleSaveSettings}>
          <Form.Item name="theme" label="主题"><Select options={[{ value: 'dark', label: '暗色' }, { value: 'light', label: '亮色' }]} /></Form.Item>
          <Form.Item name="fontSize" label="终端字体大小"><Input type="number" /></Form.Item>
          <Form.Item name="fontFamily" label="终端字体"><Input /></Form.Item>
          <Form.Item name="encoding" label="默认编码"><Select options={[{ value: 'utf-8', label: 'UTF-8' }, { value: 'gbk', label: 'GBK' }]} /></Form.Item>
          <Form.Item name="keepalive_interval" label="SSH Keepalive 间隔(秒)"><Input type="number" /></Form.Item>
          <Form.Item name="legacy_algorithms" label="启用老旧算法" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item><Button type="primary" htmlType="submit" icon={<SaveOutlined />}>保存设置</Button></Form.Item>
        </Form>
      </Card>

      <Card title="修改密码" style={{ marginBottom: 16 }}>
        <Form layout="vertical" onFinish={handleChangePassword}>
          <Form.Item name="oldPassword" label="旧密码" rules={[{ required: true }]}><Input.Password /></Form.Item>
          <Form.Item name="newPassword" label="新密码" rules={[{ required: true, min: 6 }]}><Input.Password /></Form.Item>
          <Form.Item><Button type="primary" htmlType="submit">修改密码</Button></Form.Item>
        </Form>
      </Card>

      <Card title="应用锁" style={{ marginBottom: 16 }}>
        <AppLockSection />
      </Card>

      <Card title="云同步" style={{ marginBottom: 16 }}>
        <Form form={syncForm} layout="vertical" onFinish={handleSaveSync}>
          <Form.Item name="provider" label="同步方式"><Select options={[{ value: 'local', label: '本地目录' }, { value: 'webdav', label: 'WebDAV' }, { value: 's3', label: 'S3' }, { value: 'icloud', label: 'iCloud' }]} /></Form.Item>
          <Form.Item name="enabled" label="启用自动同步" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="interval" label="同步间隔(秒)"><Input type="number" /></Form.Item>
          <Form.Item><Space><Button type="primary" htmlType="submit" icon={<SaveOutlined />}>保存</Button><Button icon={<SyncOutlined />} onClick={handleTriggerSync}>立即同步</Button></Space></Form.Item>
        </Form>
      </Card>

      <Card title="API Token (CLI 使用)">
        <Button icon={<PlusOutlined />} onClick={handleCreateToken} style={{ marginBottom: 16 }}>创建 Token</Button>
        <Table dataSource={tokens} rowKey="id" size="small" pagination={false} columns={[
          { title: '名称', dataIndex: 'name', key: 'name' },
          { title: 'Token', dataIndex: 'token', key: 'token', render: (t: string) => <Tag>{t}</Tag> },
          { title: '操作', key: 'action', width: 80, render: (_: any, r: any) => <Popconfirm title="确定删除?" onConfirm={() => handleDeleteToken(r.id)}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm> },
        ]} />
      </Card>
    </div>
  )
}

export default Settings
