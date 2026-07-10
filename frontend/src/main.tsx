import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: '#2d4a7c',
          borderRadius: 4,
          colorBgContainer: '#1a1a1a',
          colorBgLayout: '#0f0f0f',
        },
      }}
    >
      <App />
    </ConfigProvider>
  </React.StrictMode>,
)
