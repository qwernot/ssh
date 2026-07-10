import React, { useState, useEffect, useRef } from 'react'
import { Card, Table, Button, Modal, Form, Input, InputNumber, Select, Space, Popconfirm, message, Tag, Tooltip, Badge, Statistic } from 'antd'
import { PlusOutlined, DeleteOutlined, PlayCircleOutlined, StopOutlined, DashboardOutlined, ReloadOutlined } from '@ant-design/icons'
import { portForwardApi, assetApi } from '../services/api'

/** Format bytes to human-readable */
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

/** Format uptime duration */
function formatUptime(startedAt: string): string {
  const start = new Date(startedAt).getTime()
  const diff = Math.floor((Date.now() - start) / 1000)
  if (diff < 60) return `${diff}秒`
  if (diff < 3600) return `${Math.floor(diff / 60)}分${diff % 60}秒`
  const h = Math.floor(diff / 3600)
  const m = Math.floor((diff % 3600) / 60)
  return `${h}时${m}分`
}

const PortForward: React.FC = () => {
  const [rules, setRules] = useState<any[]>([])
  const [assets, setAssets] = useState<any[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()
  const [statusData, setStatusData] = useState<Record<string, any>>({})
  const [loadingStatus, setLoadingStatus] = useState(false)
  const pollRef = useRef<any>(null)

  const fetchRules = () => {
    portForwardApi.list().then(r => setRules(r.data || [])).catch(() => {})
  }

  const fetchStatus = () => {
    portForwardApi.status().then(r => setStatusData(r.data || {})).catch(() => {})
  }

  useEffect(() => {
    fetchRules()
    assetApi.list().then(r => setAssets(r.data || [])).catch(() => {})
    fetchStatus()

    // Poll status every 5 seconds
    pollRef.current = setInterval(fetchStatus, 5000)
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [])

  const handleCreate = async (values: any) => {
    await portForwardApi.create(values)
    message.success('规则已创建')
    setModalOpen(false)
    form.resetFields()
    fetchRules()
  }

  const handleDelete = async (id: number) => {
    await portForwardApi.delete(id)
    message.success('已删除')
    fetchRules()
  }

  const handleStart = async (id: number) => {
    try {
      await portForwardApi.start(id)
      message.success('转发已启动')
      fetchStatus()
    } catch (err: any) { message.error(err.message) }
  }

  const handleStop = async (id: number) => {
    try {
      await portForwardApi.stop(id)
      message.success('转发已停止')
      fetchStatus()
    } catch (err: any) { message.error(err.message) }
  }

  const isRunning = (id: number) => !!statusData[String(id)]

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => <Tag color={t === 'local' ? 'blue' : 'green'}>{t === 'local' ? '本地转发' : '远程转发'}</Tag> },
    { title: '绑定', key: 'bind', render: (_: any, r: any) => `${r.bind_host}:${r.bind_port}` },
    { title: '目标', key: 'remote', render: (_: any, r: any) => `${r.remote_host}:${r.remote_port}` },
    {
      title: '状态', key: 'status', width: 100,
      render: (_: any, r: any) => {
        const st = statusData[String(r.id)]
        if (!st) return <Badge status="default" text="未运行" />
        return (
          <Tooltip title={`活跃连接: ${st.active_conns} | 总连接: ${st.total_conns}`}>
            <Badge status="processing" text={<span style={{ color: '#52c41a' }}>运行中</span>} />
          </Tooltip>
        )
      },
    },
    {
      title: '流量', key: 'traffic', width: 180,
      render: (_: any, r: any) => {
        const st = statusData[String(r.id)]
        if (!st) return <span style={{ color: '#666' }}>-</span>
        return (
          <Space size={12}>
            <span style={{ color: '#52c41a', fontSize: 12 }}>↑ {formatBytes(st.bytes_sent)}</span>
            <span style={{ color: '#1890ff', fontSize: 12 }}>↓ {formatBytes(st.bytes_received)}</span>
          </Space>
        )
      },
    },
    {
      title: '连接', key: 'conns', width: 80,
      render: (_: any, r: any) => {
        const st = statusData[String(r.id)]
        if (!st) return <span style={{ color: '#666' }}>-</span>
        return <span>{st.active_conns}/{st.total_conns}</span>
      },
    },
    {
      title: '运行时间', key: 'uptime', width: 100,
      render: (_: any, r: any) => {
        const st = statusData[String(r.id)]
        if (!st) return <span style={{ color: '#666' }}>-</span>
        return <span style={{ fontSize: 12 }}>{formatUptime(st.started_at)}</span>
      },
    },
    {
      title: '操作', key: 'action', width: 180,
      render: (_: any, record: any) => {
        const running = isRunning(record.id)
        return (
          <Space>
            {running ? (
              <Popconfirm title="确定停止转发?" onConfirm={() => handleStop(record.id)}>
                <Button size="small" danger icon={<StopOutlined />}>停止</Button>
              </Popconfirm>
            ) : (
              <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => handleStart(record.id)}>启动</Button>
            )}
            <Popconfirm title="确定删除?" onConfirm={() => handleDelete(record.id)}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Popconfirm>
          </Space>
        )
      },
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><DashboardOutlined /> 端口转发规则</>}
        extra={
          <Space>
            <Button size="small" icon={<ReloadOutlined />} onClick={fetchStatus}>刷新状态</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>新增规则</Button>
          </Space>
        }>
        {/* Summary stats */}
        {Object.keys(statusData).length > 0 && (
          <div style={{ marginBottom: 16, display: 'flex', gap: 24 }}>
            <Statistic title="活跃转发" value={Object.keys(statusData).length} suffix="条" valueStyle={{ fontSize: 18 }} />
            <Statistic title="总连接数" value={Object.values(statusData).reduce((s: number, v: any) => s + (v.total_conns || 0), 0)} valueStyle={{ fontSize: 18 }} />
            <Statistic title="总流量" value={formatBytes(Object.values(statusData).reduce((s: number, v: any) => s + (v.bytes_sent || 0) + (v.bytes_received || 0), 0))} valueStyle={{ fontSize: 18 }} />
          </div>
        )}
        <Table columns={columns} dataSource={rules} rowKey="id" size="small" pagination={false} />
      </Card>
      <Modal title="新增转发规则" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ type: 'local', bind_host: '127.0.0.1', remote_host: '127.0.0.1' }}>
          <Form.Item name="name" label="名称"><Input /></Form.Item>
          <Form.Item name="asset_id" label="关联资产" rules={[{ required: true }]}>
            <Select options={assets.map(a => ({ value: a.id, label: `${a.name} (${a.host})` }))} />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={[{ value: 'local', label: '本地转发' }, { value: 'remote', label: '远程转发' }]} />
          </Form.Item>
          <Space>
            <Form.Item name="bind_host" label="绑定地址"><Input style={{ width: 150 }} /></Form.Item>
            <Form.Item name="bind_port" label="绑定端口" rules={[{ required: true }]}><InputNumber min={1} max={65535} /></Form.Item>
          </Space>
          <Space>
            <Form.Item name="remote_host" label="目标地址"><Input style={{ width: 150 }} /></Form.Item>
            <Form.Item name="remote_port" label="目标端口" rules={[{ required: true }]}><InputNumber min={1} max={65535} /></Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}

export default PortForward
