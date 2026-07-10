# Shelly - SSH 终端管理工具

一站式 SSH/Telnet/Serial 终端管理平台，支持资产管理、多标签终端、SFTP 文件传输、批量执行、端口转发、会话录制回放、AI 助手等功能。

## 功能特性

- **多协议连接**: SSH / Telnet / Serial / Local Shell
- **资产管理**: 分组、标签、搜索、加密存储密码与私钥 (AES-256-GCM)
- **终端增强**: 多标签、水平/垂直分屏、关键字搜索、命令片段、关键字高亮
- **SFTP 文件传输**: 双面板浏览、文件夹上传、批量 ZIP 下载、ZModem 支持
- **批量执行**: 并发 SSH 执行，每台机器独立输出面板
- **端口转发**: 本地/远程 SSH 隧道，流量统计与状态监控
- **会话录制**: asciicast v2 格式，xterm.js 回放，支持倍速/拖拽进度条
- **AI 助手**: OpenAI 兼容 API，SSE 流式响应，命令确认机制
- **云同步**: WebDAV / S3 / iCloud / 本地目录
- **安全**: 应用锁 PIN + 超时锁定、JWT + API Token 双认证
- **编码**: GBK / UTF-8 自动切换

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.22 / Gin / GORM / SQLite |
| 前端 | React 18 / TypeScript / Vite / Ant Design / xterm.js 5.x |
| 通信 | WebSocket (终端) + REST API (管理) + SSE (AI) |
| 部署 | Docker / 单二进制 |

## Docker 部署

### 快速启动

```bash
# 克隆项目
git clone <your-repo-url> shelly
cd shelly

# 修改配置（重要：修改 JWT secret 和 crypto key）
vim backend/config.yaml

# 构建并启动
docker compose up -d --build
```

启动后访问 `http://localhost:8080`，首次使用需注册账号。

### 配置说明

编辑 `backend/config.yaml`：

```yaml
server:
  host: 0.0.0.0
  port: 8080
  mode: release

database:
  driver: sqlite              # sqlite 或 postgres
  dsn: data/shelly.db         # SQLite 文件路径

jwt:
  secret: your-secure-secret  # ⚠️ 生产环境务必修改
  expire: 72                  # Token 过期时间（小时）

crypto:
  key: your-32-byte-hex-key   # ⚠️ 生产环境务必修改（64位hex = 32字节）

ssh:
  keepalive_interval: 30      # SSH 心跳间隔（秒）
  keepalive_count: 3          # 心跳失败次数上限
  legacy_algorithms: false    # 是否启用老旧 SSH 算法

session:
  record_dir: data/recordings # 会话录像存储目录

ai:
  provider: openai            # openai / anthropic
  api_key: "sk-xxx"           # AI API Key
  base_url: ""                # 自定义 API 地址（可选）
  model: gpt-4
  max_context_lines: 200      # AI 上下文捕获行数

sync:
  enabled: false
  provider: local             # local / webdav / s3 / icloud
  interval: 300               # 自动同步间隔（秒）
  config: {}
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  shelly:
    build: .
    container_name: shelly
    ports:
      - "8080:8080"
    volumes:
      - ./backend/config.yaml:/app/config.yaml
      - shelly-data:/app/data
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped

volumes:
  shelly-data:
    driver: local
```

### 数据持久化

Docker 部署通过 `shelly-data` 卷持久化以下数据：

- `data/shelly.db` — SQLite 数据库
- `data/recordings/` — 会话录像文件

### 常用命令

```bash
# 查看日志
docker compose logs -f shelly

# 重新构建
docker compose up -d --build

# 停止
docker compose down

# 备份数据
docker compose exec shelly tar czf /tmp/backup.tar.gz /app/data
docker cp shelly:/tmp/backup.tar.gz ./backup.tar.gz
```

## 单二进制部署（无 Docker）

```bash
# PowerShell (Windows)
.\build.ps1

# 或直接执行
.\shelly.exe
```

`build.ps1` 会自动完成：前端构建 → 复制到 embed 目录 → 后端编译 → 输出单二进制 `shelly.exe`。

运行后访问 `http://localhost:8080`。

## 项目结构

```
shelly/
├── Dockerfile                  # 多阶段构建（前端+后端+alpine运行时）
├── docker-compose.yml
├── build.ps1                   # Windows 一键构建脚本
├── backend/
│   ├── cmd/
│   │   ├── server/             # 服务端入口 + embed 前端
│   │   └── cli/                # CLI 工具 (shelly-cli)
│   ├── internal/
│   │   ├── api/                # HTTP/WebSocket handlers
│   │   ├── model/              # GORM 数据模型
│   │   ├── ssh/                # SSH/Telnet/Serial 客户端
│   │   ├── sync/               # 云同步引擎
│   │   ├── middleware/         # JWT 认证中间件
│   │   ├── config/             # 配置加载
│   │   ├── database/           # 数据库初始化
│   │   └── websocket/          # WebSocket Hub
│   ├── pkg/
│   │   ├── asciicast/          # asciicast v2 格式
│   │   ├── crypto/             # AES-256-GCM 加密
│   │   └── keepalive/          # SSH keepalive
│   └── config.yaml
└── frontend/
    ├── Dockerfile              # 前端独立构建（nginx）
    ├── nginx.conf
    └── src/
        ├── components/         # UI 组件
        ├── stores/             # Zustand 状态管理
        ├── services/           # API 调用层
        └── hooks/              # 自定义 Hooks
```

## CLI 工具

```bash
# 使用 API Token 认证（在设置页面生成 Token）
shelly-cli asset list
shelly-cli asset get <id>
shelly-cli exec <asset-name> "uptime"
shelly-cli upload <asset-name> ./local-file /remote/path
shelly-cli download <asset-name> /remote/file ./local-dir
```

## License

MIT
