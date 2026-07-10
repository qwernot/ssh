import React, { useState } from 'react'
import { Layout, Menu } from 'antd'
import {
  DesktopOutlined, CodeOutlined, SwapOutlined, PlayCircleOutlined,
  RobotOutlined, CloudSyncOutlined, SettingOutlined, LogoutOutlined,
  FolderOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import { useAuthStore } from '../stores/authStore'
import AssetManager from './AssetManager'
import TerminalView from './TerminalView'
import FileTransfer from './FileTransfer'
import BatchExec from './BatchExec'
import SessionPlayer from './SessionPlayer'
import AIAssistant from './AIAssistant'
import PortForward from './PortForward'
import Settings from './Settings'

const { Sider, Content } = Layout

const menuItems = [
  { key: 'assets', icon: <DesktopOutlined />, label: '资产管理' },
  { key: 'terminal', icon: <CodeOutlined />, label: '终端' },
  { key: 'filetransfer', icon: <FolderOutlined />, label: '文件传输' },
  { key: 'batchexec', icon: <ThunderboltOutlined />, label: '批量执行' },
  { key: 'portforward', icon: <SwapOutlined />, label: '端口转发' },
  { key: 'sessions', icon: <PlayCircleOutlined />, label: '会话回放' },
  { key: 'ai', icon: <RobotOutlined />, label: 'AI 助手' },
  { key: 'sync', icon: <CloudSyncOutlined />, label: '同步' },
  { key: 'settings', icon: <SettingOutlined />, label: '设置' },
]

const MainLayout: React.FC = () => {
  const [activeView, setActiveView] = useState('assets')
  const [collapsed, setCollapsed] = useState(false)
  const logout = useAuthStore((s) => s.logout)

  const renderContent = () => {
    switch (activeView) {
      case 'assets': return <AssetManager />
      case 'terminal': return <TerminalView />
      case 'filetransfer': return <FileTransfer />
      case 'batchexec': return <BatchExec />
      case 'portforward': return <PortForward />
      case 'sessions': return <SessionPlayer />
      case 'ai': return <AIAssistant />
      case 'sync': return <Settings />
      case 'settings': return <Settings />
      default: return <AssetManager />
    }
  }

  return (
    <Layout style={{ height: '100vh' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        theme="dark"
        width={200}
      >
        <div style={{ height: 48, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: collapsed ? 14 : 18, fontWeight: 'bold' }}>
          {collapsed ? 'S' : 'Shelly'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[activeView]}
          items={menuItems}
          onClick={({ key }) => setActiveView(key)}
        />
        <Menu
          theme="dark"
          mode="inline"
          style={{ position: 'absolute', bottom: 0, width: '100%' }}
          items={[{ key: 'logout', icon: <LogoutOutlined />, label: '退出登录' }]}
          onClick={logout}
        />
      </Sider>
      <Layout>
        <Content style={{ overflow: 'auto', background: '#0f0f0f' }}>
          {renderContent()}
        </Content>
      </Layout>
    </Layout>
  )
}

export default MainLayout
