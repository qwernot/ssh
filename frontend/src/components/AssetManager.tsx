import React, { useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, Select, InputNumber, Tag, Space, Tree, message, Popconfirm, Card, TreeSelect, Divider } from 'antd'
import { PlusOutlined, DeleteOutlined, EditOutlined, SearchOutlined, ApiOutlined, FolderOutlined, AppstoreOutlined } from '@ant-design/icons'
import { assetApi, groupApi } from '../services/api'
import { useTerminalStore, AssetInfo } from '../stores/terminalStore'

const AssetManager: React.FC = () => {
  const [assets, setAssets] = useState<any[]>([])
  const [groups, setGroups] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [groupModalOpen, setGroupModalOpen] = useState(false)
  const [editingAsset, setEditingAsset] = useState<any>(null)
  const [editingGroup, setEditingGroup] = useState<any>(null)
  const [search, setSearch] = useState('')
  const [selectedGroupId, setSelectedGroupId] = useState<number | null>(null)
  const [form] = Form.useForm()
  const [groupForm] = Form.useForm()
  const addTab = useTerminalStore((s) => s.addTab)

  const loadData = async () => {
    setLoading(true)
    try {
      const params: any = {}
      if (search) params.search = search
      if (selectedGroupId) params.group_id = String(selectedGroupId)
      const [assetRes, groupRes] = await Promise.all([
        assetApi.list(params),
        groupApi.list(),
      ])
      setAssets(assetRes.data || [])
      setGroups(groupRes.data || [])
    } catch (e) { /* ignore */ }
    setLoading(false)
  }

  useEffect(() => { loadData() }, [search, selectedGroupId])

  const handleSave = async (values: any) => {
    try {
      if (editingAsset) {
        await assetApi.update(editingAsset.id, values)
        message.success('更新成功')
      } else {
        await assetApi.create(values)
        message.success('创建成功')
      }
      setModalOpen(false)
      form.resetFields()
      setEditingAsset(null)
      loadData()
    } catch (err: any) {
      message.error(err.message)
    }
  }

  const handleDelete = async (id: number) => {
    await assetApi.delete(id)
    message.success('已删除')
    loadData()
  }

  const handleConnect = (record: any) => {
    addTab(record as AssetInfo)
    message.success(`正在连接 ${record.name}`)
  }

  const handleSaveGroup = async (values: any) => {
    try {
      if (editingGroup) {
        await groupApi.update(editingGroup.id, values)
        message.success('分组已更新')
      } else {
        await groupApi.create(values)
        message.success('分组已创建')
      }
      setGroupModalOpen(false)
      groupForm.resetFields()
      setEditingGroup(null)
      loadData()
    } catch (err: any) {
      message.error(err.message)
    }
  }

  const handleDeleteGroup = async (id: number) => {
    await groupApi.delete(id)
    message.success('分组已删除')
    if (selectedGroupId === id) setSelectedGroupId(null)
    loadData()
  }

  // Build tree data for groups
  const buildGroupTree = () => {
    const map: Record<number, any> = {}
    const roots: any[] = []
    groups.forEach(g => { map[g.id] = { ...g, children: [] } })
    groups.forEach(g => {
      if (g.parent_id && map[g.parent_id]) {
        map[g.parent_id].children.push(map[g.id])
      } else {
        roots.push(map[g.id])
      }
    })
    return roots
  }

  const groupTreeData = buildGroupTree()

  const renderTreeNodes = (data: any[]): any[] =>
    data.map(item => ({
      title: <span><FolderOutlined style={{ marginRight: 4 }} />{item.name}</span>,
      key: item.id,
      value: item.id,
      children: item.children?.length ? renderTreeNodes(item.children) : undefined,
    }))

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', width: 150 },
    { title: '类型', dataIndex: 'type', key: 'type', width: 80, render: (t: string) => <Tag color={t === 'ssh' ? 'blue' : t === 'telnet' ? 'green' : 'orange'}>{t.toUpperCase()}</Tag> },
    { title: '主机', dataIndex: 'host', key: 'host' },
    { title: '端口', dataIndex: 'port', key: 'port', width: 70 },
    { title: '用户', dataIndex: 'username', key: 'username', width: 100 },
    { title: '标签', dataIndex: 'tags', key: 'tags', render: (tags: string) => tags ? tags.split(',').map(t => <Tag key={t}>{t}</Tag>) : null },
    {
      title: '操作', key: 'action', width: 200,
      render: (_: any, record: any) => (
        <Space>
          <Button type="link" icon={<ApiOutlined />} onClick={() => handleConnect(record)}>连接</Button>
          <Button type="link" icon={<EditOutlined />} onClick={() => { setEditingAsset(record); form.setFieldsValue(record); setModalOpen(true) }}>编辑</Button>
          <Popconfirm title="确定删除?" onConfirm={() => handleDelete(record.id)}>
            <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ display: 'flex', gap: 16, padding: 24, height: '100%' }}>
      {/* Left: Group tree */}
      <Card
        title={<><AppstoreOutlined /> 资产分组</>}
        style={{ width: 240, flexShrink: 0, overflow: 'auto' }}
        bodyStyle={{ padding: '8px 0' }}
        extra={
          <Button size="small" icon={<PlusOutlined />} onClick={() => { setEditingGroup(null); groupForm.resetFields(); setGroupModalOpen(true) }}>
            新增
          </Button>
        }
      >
        <div
          style={{ padding: '4px 12px', cursor: 'pointer', borderRadius: 4, background: selectedGroupId === null ? '#2d4a7c33' : undefined }}
          onClick={() => setSelectedGroupId(null)}
        >
          <FolderOutlined style={{ marginRight: 4 }} /> 全部资产
        </div>
        {groupTreeData.length > 0 && (
          <Tree
            treeData={renderTreeNodes(groupTreeData)}
            selectedKeys={selectedGroupId ? [selectedGroupId] : []}
            onSelect={(keys) => setSelectedGroupId(keys[0] as number || null)}
            style={{ marginTop: 4 }}
            blockNode
          />
        )}
      </Card>

      {/* Right: Asset table */}
      <Card style={{ flex: 1, overflow: 'auto' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
          <Space>
            <Input prefix={<SearchOutlined />} placeholder="搜索资产..." value={search} onChange={e => setSearch(e.target.value)} style={{ width: 250 }} allowClear />
          </Space>
          <Space>
            <Button icon={<PlusOutlined />} onClick={() => { setEditingGroup(null); groupForm.resetFields(); setGroupModalOpen(true) }}>管理分组</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingAsset(null); form.resetFields(); setModalOpen(true) }}>新增资产</Button>
          </Space>
        </div>
        <Table columns={columns} dataSource={assets} rowKey="id" loading={loading} size="small" pagination={{ pageSize: 20 }} />
      </Card>

      {/* Asset Modal */}
      <Modal
        title={editingAsset ? '编辑资产' : '新增资产'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleSave} initialValues={{ type: 'ssh', port: 22, auth_type: 'password', keepalive_interval: 30, keepalive_count: 3, encoding: 'utf-8' }}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={[{ value: 'ssh', label: 'SSH' }, { value: 'telnet', label: 'Telnet' }, { value: 'serial', label: '串口' }, { value: 'local', label: '本地终端' }, { value: 'nextterminal', label: 'NextTerminal' }]} />
          </Form.Item>
          <Form.Item name="host" label="主机"><Input /></Form.Item>
          <Form.Item name="port" label="端口"><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="username" label="用户名"><Input /></Form.Item>
          <Form.Item name="auth_type" label="认证方式">
            <Select options={[{ value: 'password', label: '密码' }, { value: 'private_key', label: '私钥' }, { value: 'keyboard_interactive', label: 'Keyboard Interactive' }]} />
          </Form.Item>
          <Form.Item name="password" label="密码"><Input.Password /></Form.Item>
          <Form.Item name="private_key" label="私钥"><Input.TextArea rows={3} /></Form.Item>
          <Form.Item name="group_id" label="所属分组">
            <TreeSelect
              treeData={renderTreeNodes(groupTreeData)}
              allowClear
              placeholder="选择分组"
            />
          </Form.Item>
          <Form.Item name="serial_port" label="串口"><Input placeholder="COM3 或 /dev/ttyS0" /></Form.Item>
          <Form.Item name="baud_rate" label="波特率"><InputNumber min={1200} max={921600} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="encoding" label="编码">
            <Select options={[{ value: 'utf-8', label: 'UTF-8' }, { value: 'gbk', label: 'GBK' }]} />
          </Form.Item>
          <Form.Item name="keepalive_interval" label="Keepalive 间隔(秒)"><InputNumber min={0} max={3600} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="legacy_algorithms" label="启用老旧算法" valuePropName="checked"><Select options={[{ value: true, label: '是' }, { value: false, label: '否' }]} /></Form.Item>
          <Form.Item name="tags" label="标签 (逗号分隔)"><Input placeholder="web,prod" /></Form.Item>
          <Form.Item name="note" label="备注"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>

      {/* Group Modal */}
      <Modal
        title={editingGroup ? '编辑分组' : '新增分组'}
        open={groupModalOpen}
        onCancel={() => setGroupModalOpen(false)}
        onOk={() => groupForm.submit()}
      >
        <Form form={groupForm} layout="vertical" onFinish={handleSaveGroup}>
          <Form.Item name="name" label="分组名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="parent_id" label="父分组">
            <TreeSelect treeData={renderTreeNodes(groupTreeData)} allowClear placeholder="无 (顶级分组)" />
          </Form.Item>
        </Form>
        <Divider />
        <div style={{ maxHeight: 200, overflow: 'auto' }}>
          {groups.map(g => (
            <div key={g.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '4px 0' }}>
              <span><FolderOutlined style={{ marginRight: 8 }} />{g.name}</span>
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={() => { setEditingGroup(g); groupForm.setFieldsValue(g); setGroupModalOpen(true) }} />
                <Popconfirm title="确定删除分组?" onConfirm={() => handleDeleteGroup(g.id)}>
                  <Button size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>
              </Space>
            </div>
          ))}
        </div>
      </Modal>
    </div>
  )
}

export default AssetManager
