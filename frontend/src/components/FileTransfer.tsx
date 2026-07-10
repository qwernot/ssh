import React, { useState, useEffect, useCallback, useRef } from 'react'
import { Card, Select, Table, Button, Space, Input, Breadcrumb, Upload, Modal, Form, message, Popconfirm, Progress, Tag, Tooltip } from 'antd'
import { FolderOutlined, FileOutlined, UploadOutlined, DownloadOutlined, DeleteOutlined, PlusOutlined, HomeOutlined, SwapOutlined, InboxOutlined, FolderAddOutlined } from '@ant-design/icons'
import { assetApi, sftpApi } from '../services/api'
import { useTerminalStore } from '../stores/terminalStore'

const { Dragger } = Upload

interface FileEntry {
  name: string
  size: number
  is_dir: boolean
  mode: string
  mod_time: number
}

const FileTransfer: React.FC = () => {
  const [assets, setAssets] = useState<any[]>([])
  const [selectedAsset, setSelectedAsset] = useState<number | null>(null)
  const [remotePath, setRemotePath] = useState('/')
  const [remoteFiles, setRemoteFiles] = useState<FileEntry[]>([])
  const [localPath, setLocalPath] = useState('')
  const [loading, setLoading] = useState(false)
  const [mkdirOpen, setMkdirOpen] = useState(false)
  const [newDirName, setNewDirName] = useState('')
  const [uploadProgress, setUploadProgress] = useState<{ name: string; percent: number }[]>([])
  const [selectedRemoteKeys, setSelectedRemoteKeys] = useState<React.Key[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)
  const folderInputRef = useRef<HTMLInputElement>(null)

  // Directory following: watch active terminal for cd commands
  const tabs = useTerminalStore((s) => s.tabs)
  const activeTabId = useTerminalStore((s) => s.activeTabId)
  const activeTab = tabs.find(t => t.id === activeTabId)

  useEffect(() => {
    if (!activeTab?.lastOutput) return
    const output = activeTab.lastOutput
    // Detect cd commands in terminal output
    const cdMatch = output.match(/\$ cd\s+(.+?)[\r\n]/g)
    if (cdMatch && cdMatch.length > 0) {
      const lastCd = cdMatch[cdMatch.length - 1]
      const dir = lastCd.replace('$ cd', '').trim()
      if (dir && selectedAsset) {
        const newPath = dir.startsWith('/') ? dir : `${remotePath === '/' ? '' : remotePath}/${dir}`
        loadFiles(selectedAsset, newPath)
      }
    }
  }, [activeTab?.lastOutput])

  useEffect(() => { assetApi.list().then(r => setAssets(r.data || [])).catch(() => {}) }, [])

  const loadFiles = async (assetId: number, path: string) => {
    setLoading(true)
    try {
      const res = await sftpApi.list(assetId, path)
      setRemoteFiles(res.files || [])
      setRemotePath(path)
    } catch (err: any) { message.error(err.message) }
    setLoading(false)
  }

  const handleSingleUpload = async (file: File) => {
    if (!selectedAsset) return false
    setUploadProgress(prev => [...prev, { name: file.name, percent: 0 }])
    try {
      await sftpApi.upload(selectedAsset, file, remotePath)
      setUploadProgress(prev => prev.map(p => p.name === file.name ? { ...p, percent: 100 } : p))
      message.success(`${file.name} 上传成功`)
      loadFiles(selectedAsset, remotePath)
    } catch (err: any) {
      message.error(`${file.name}: ${err.message}`)
    }
    setTimeout(() => setUploadProgress(prev => prev.filter(p => p.name !== file.name)), 2000)
    return false
  }

  const handleFolderUpload = async (fileList: FileList) => {
    if (!selectedAsset || !fileList.length) return
    // Get the folder name from the first file's webkitRelativePath
    const firstFile = fileList[0]
    const folderName = firstFile.webkitRelativePath.split('/')[0]
    const targetPath = `${remotePath === '/' ? '' : remotePath}/${folderName}`

    // Create remote folder first
    await sftpApi.mkdir(selectedAsset, targetPath)

    // Upload all files
    const total = fileList.length
    let done = 0
    for (let i = 0; i < fileList.length; i++) {
      const file = fileList[i]
      const relativePath = file.webkitRelativePath
      const subPath = relativePath.substring(folderName.length)
      const destDir = `${targetPath}${subPath.substring(0, subPath.lastIndexOf('/'))}`

      // Create subdirectories if needed
      if (destDir && destDir !== targetPath) {
        await sftpApi.mkdir(selectedAsset, destDir)
      }
      await sftpApi.upload(selectedAsset, file, destDir || targetPath)
      done++
      setUploadProgress([{ name: `文件夹上传 (${done}/${total})`, percent: Math.round(done / total * 100) }])
    }

    message.success(`文件夹 ${folderName} 上传完成`)
    setUploadProgress([])
    loadFiles(selectedAsset, remotePath)
  }

  const handleBatchDownload = async () => {
    if (!selectedAsset || !selectedRemoteKeys.length) return
    const paths = selectedRemoteKeys.map(k => `${remotePath === '/' ? '' : remotePath}/${k}`)
    try {
      const res = await fetch(`/api/sftp/${selectedAsset}/batch-download`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${localStorage.getItem('token')}` },
        body: JSON.stringify({ paths })
      })
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'files.zip'
      a.click()
      URL.revokeObjectURL(url)
      message.success('批量下载已开始')
    } catch (err: any) { message.error(err.message) }
  }

  const handleDelete = async (paths: string[]) => {
    if (!selectedAsset) return
    await sftpApi.delete(selectedAsset, paths)
    message.success('已删除')
    loadFiles(selectedAsset, remotePath)
  }

  const handleMkdir = async () => {
    if (!selectedAsset || !newDirName) return
    await sftpApi.mkdir(selectedAsset, `${remotePath === '/' ? '' : remotePath}/${newDirName}`)
    message.success('目录已创建')
    setMkdirOpen(false)
    setNewDirName('')
    loadFiles(selectedAsset, remotePath)
  }

  const navigateTo = (name: string) => {
    if (!selectedAsset) return
    const newPath = `${remotePath === '/' ? '' : remotePath}/${name}`
    loadFiles(selectedAsset, newPath)
  }

  const columns = [
    {
      title: '名称', dataIndex: 'name', key: 'name',
      render: (name: string, record: FileEntry) => (
        <span onClick={() => record.is_dir && navigateTo(name)} style={{ cursor: record.is_dir ? 'pointer' : 'default' }}>
          {record.is_dir ? <FolderOutlined style={{ color: '#faad14', marginRight: 8 }} /> : <FileOutlined style={{ marginRight: 8 }} />}
          {name}
        </span>
      ),
    },
    { title: '大小', dataIndex: 'size', key: 'size', width: 100, render: (size: number) => size > 1024 * 1024 ? `${(size / 1024 / 1024).toFixed(1)}MB` : size > 1024 ? `${(size / 1024).toFixed(1)}KB` : `${size}B` },
    { title: '权限', dataIndex: 'mode', key: 'mode', width: 120 },
    { title: '修改时间', dataIndex: 'mod_time', key: 'mod_time', width: 180, render: (t: number) => new Date(t * 1000).toLocaleString() },
  ]

  return (
    <div style={{ padding: 24, height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* Asset selector */}
      <Space style={{ marginBottom: 16 }}>
        <Select placeholder="选择资产" style={{ width: 250 }} onChange={(v) => { setSelectedAsset(v); loadFiles(v, '/') }}
          options={assets.map(a => ({ value: a.id, label: `${a.name} (${a.host})` }))} />
        {activeTab && <Tag color="blue">跟随终端: {activeTab.asset.name}</Tag>}
      </Space>

      <div style={{ display: 'flex', gap: 16, flex: 1, minHeight: 0 }}>
        {/* Left panel: Local files (upload area) */}
        <Card title="本地文件" style={{ flex: 1, display: 'flex', flexDirection: 'column' }}
          bodyStyle={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'auto' }}>
          <Space style={{ marginBottom: 12 }}>
            <Button icon={<UploadOutlined />} onClick={() => fileInputRef.current?.click()}>上传文件</Button>
            <Button icon={<FolderAddOutlined />} onClick={() => folderInputRef.current?.click()}>上传文件夹</Button>
          </Space>
          <input ref={fileInputRef} type="file" multiple style={{ display: 'none' }}
            onChange={e => { if (e.target.files) Array.from(e.target.files).forEach(f => handleSingleUpload(f)); e.target.value = '' }} />
          <input ref={folderInputRef} type="file" {...{ webkitdirectory: '', directory: '' } } style={{ display: 'none' }}
            onChange={e => { if (e.target.files) handleFolderUpload(e.target.files); e.target.value = '' }} />
          <Dragger
            multiple
            showUploadList={false}
            beforeUpload={(file) => { handleSingleUpload(file); return false }}
            style={{ flex: 1 }}
          >
            <p className="ant-upload-drag-icon"><InboxOutlined /></p>
            <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
            <p className="ant-upload-hint">支持单文件/多文件/文件夹上传</p>
          </Dragger>
          {uploadProgress.length > 0 && (
            <div style={{ marginTop: 12 }}>
              {uploadProgress.map((p, i) => (
                <div key={i} style={{ marginBottom: 4 }}>
                  <span style={{ fontSize: 12 }}>{p.name}</span>
                  <Progress percent={p.percent} size="small" />
                </div>
              ))}
            </div>
          )}
        </Card>

        {/* Right panel: Remote SFTP */}
        <Card title="远程文件" style={{ flex: 1, display: 'flex', flexDirection: 'column' }}
          bodyStyle={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'auto', padding: 12 }}>
          <Space style={{ marginBottom: 12, width: '100%', justifyContent: 'space-between' }} wrap>
            <Space>
              <Breadcrumb items={[
                { title: <HomeOutlined onClick={() => selectedAsset && loadFiles(selectedAsset, '/')} style={{ cursor: 'pointer' }} /> },
                ...remotePath.split('/').filter(Boolean).map((p, i) => ({
                  title: <span onClick={() => { if (selectedAsset) loadFiles(selectedAsset, '/' + remotePath.split('/').filter(Boolean).slice(0, i + 1).join('/')) }} style={{ cursor: 'pointer' }}>{p}</span>
                }))
              ]} />
            </Space>
            <Space>
              <Tooltip title="批量下载为ZIP">
                <Button icon={<DownloadOutlined />} disabled={!selectedRemoteKeys.length} onClick={handleBatchDownload}>
                  下载{selectedRemoteKeys.length > 0 ? `(${selectedRemoteKeys.length})` : ''}
                </Button>
              </Tooltip>
              <Button icon={<PlusOutlined />} onClick={() => setMkdirOpen(true)}>新建目录</Button>
              <Popconfirm title="确定删除选中文件?" onConfirm={() => handleDelete(selectedRemoteKeys.map(k => `${remotePath === '/' ? '' : remotePath}/${k}`))}>
                <Button icon={<DeleteOutlined />} danger disabled={!selectedRemoteKeys.length}>删除</Button>
              </Popconfirm>
            </Space>
          </Space>
          <Table columns={columns} dataSource={remoteFiles} rowKey="name" loading={loading} size="small" pagination={false}
            scroll={{ y: 'calc(100vh - 380px)' }}
            rowSelection={{
              type: 'checkbox',
              selectedRowKeys: selectedRemoteKeys,
              onChange: (keys) => setSelectedRemoteKeys(keys),
            }}
          />
        </Card>
      </div>

      <Modal title="新建目录" open={mkdirOpen} onOk={handleMkdir} onCancel={() => setMkdirOpen(false)}>
        <Input value={newDirName} onChange={e => setNewDirName(e.target.value)} placeholder="目录名" />
      </Modal>
    </div>
  )
}

export default FileTransfer
