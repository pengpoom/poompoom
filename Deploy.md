# Image Studio 上线配置说明

更新时间: 2026-05-18

本文档记录当前单机小范围上线所需的启动命令、存储路径和配置方式。当前建议先使用 SQLite + 本地图片目录，不强制引入 Redis 或 PostgreSQL。

## 1. 目录结构

当前主要代码目录:

```text
/home/peng/Project/image/image-studio
├── web/backend        # Go 业务后端
├── web/web            # Vite 前端
├── Interface.md       # 接口文档
├── update.md          # 开发进度记录
└── Deploy.md          # 本文档
```

后端运行目录必须是:

```bash
cd "/home/peng/Project/image/image-studio/web/backend"
```

前端运行目录必须是:

```bash
cd "/home/peng/Project/image/image-studio/web/web"
```

## 2. 基础环境

当前代码要求:

- Go: `1.25+`
- Node.js: 建议 `20+`
- npm: 使用项目现有 `package-lock.json`
- SQLite: 默认使用后端内置 SQLite 驱动，不需要单独启动数据库服务
- Redis: 当前不是必须

检查命令:

```bash
go version
node -v
npm -v
```

## 3. 本地开发启动

本地开发建议前后端分开启动。

### 3.1 启动后端

```bash
cd "/home/peng/Project/image/image-studio/web/backend"

API_ONLY=true \
SERVER_HOST=0.0.0.0 \
SERVER_PORT=7070 \
APP_SESSION_TTL_HOURS=24 \
go run .
```

含义:

- `API_ONLY=true`: 只启动 API，不要求后端存在前端静态文件
- `SERVER_HOST=0.0.0.0`: 允许局域网访问后端
- `SERVER_PORT=7070`: 后端监听 `7070`，匹配前端 Vite 代理
- `APP_SESSION_TTL_HOURS=24`: 登录会话有效期 24 小时

如果看到 `address already in use`，说明 `7070` 已被占用，需要关闭旧后端进程或换端口。

### 3.2 启动前端

```bash
cd "/home/peng/Project/image/image-studio/web/web"
npm install
npm run dev
```

默认前端地址:

```text
http://localhost:5270/
```

开发模式下，Vite 会把以下请求代理到后端 `http://127.0.0.1:7070`:

- `/auth`
- `/api`
- `/version`
- `/health`
- `/v1/files/image`

所以开发模式必须保证后端运行在 `7070`，除非同步修改 `web/web/vite.config.ts`。

## 4. 小范围上线启动

小范围上线建议使用后端统一托管前端静态资源，这样只需要暴露一个后端端口。

### 4.1 构建前端并同步到后端

```bash
cd "/home/peng/Project/image/image-studio/web/web"
npm install
SYNC_BACKEND_STATIC=true npm run build
```

`SYNC_BACKEND_STATIC=true` 会把前端构建产物同步到:

```text
/home/peng/Project/image/image-studio/web/backend/static
```

### 4.2 启动后端

```bash
cd "/home/peng/Project/image/image-studio/web/backend"

SERVER_HOST=0.0.0.0 \
SERVER_PORT=7070 \
APP_SESSION_TTL_HOURS=24 \
go run .
```

上线模式不要加 `API_ONLY=true`，这样后端会同时提供:

- 前端页面
- 登录接口
- 业务 API
- 图片文件访问

访问地址:

```text
http://服务器IP:7070/
```

### 4.3 构建后端二进制，可选

如果不想每次 `go run .`，可以构建二进制:

```bash
cd "/home/peng/Project/image/image-studio/web/backend"
go build -o image-studio-backend .
```

运行:

```bash
SERVER_HOST=0.0.0.0 \
SERVER_PORT=7070 \
APP_SESSION_TTL_HOURS=24 \
./image-studio-backend
```

## 5. 配置文件

后端首次启动会自动生成运行配置文件:

```text
/home/peng/Project/image/image-studio/web/backend/data/config.toml
```

默认配置模板来自:

```text
/home/peng/Project/image/image-studio/web/backend/internal/config/config.defaults.toml
```

常用配置项:

```toml
[server]
host = "0.0.0.0"
port = 7000
max_image_concurrency = 8
image_queue_limit = 32
image_queue_timeout_seconds = 20

[storage]
backend = "current"
config_backend = "file"
image_dir = "data/business-images"
sqlite_path = "data/image-studio.db"

[chatgpt]
request_timeout = 120
```

说明:

- `server.max_image_concurrency`: 最大同时生图任务数
- `server.image_queue_limit`: 最大排队数量
- `server.image_queue_timeout_seconds`: 排队等待超时时间
- `storage.sqlite_path`: SQLite 数据库路径
- `storage.image_dir`: 图片文件存储路径
- `chatgpt.request_timeout`: 单次上游请求超时时间

环境变量会覆盖配置文件里的同名配置。当前常用环境变量:

```text
SERVER_HOST
SERVER_PORT
APP_SESSION_TTL_HOURS
STORAGE_BACKEND
STORAGE_CONFIG_BACKEND
STORAGE_IMAGE_DIR
STORAGE_SQLITE_PATH
REDIS_ADDR
REDIS_PASSWORD
REDIS_DB
REDIS_PREFIX
CHATGPT_REQUEST_TIMEOUT
API_ACCESS_PLATFORM
API_ACCESS_BASE_URL
API_ACCESS_API_KEY
```

## 6. 数据库路径

当前默认 SQLite 数据库:

```text
/home/peng/Project/image/image-studio/web/backend/data/image-studio.db
```

里面保存:

- 业务用户
- 登录会话
- 图片会话
- 生图记录
- 图片资产表
- 点数流水
- Job 记录
- API 接入配置
- 系统设置

查看数据库示例:

```bash
sqlite3 "/home/peng/Project/image/image-studio/web/backend/data/image-studio.db" ".tables"
```

查看最近 Job:

```bash
sqlite3 "/home/peng/Project/image/image-studio/web/backend/data/image-studio.db" \
"SELECT id, status, stage, provider_platform, created_at, updated_at FROM business_image_jobs ORDER BY created_at DESC LIMIT 5;"
```

## 7. 图片存储路径

当前默认图片目录:

```text
/home/peng/Project/image/image-studio/web/backend/data/business-images
```

旧版本兼容目录:

```text
/home/peng/Project/image/image-studio/web/backend/data/tmp/image
```

新生成的业务图片会保存成本地文件，数据库里保存对应 URL，例如:

```text
/v1/files/image/business-user_xxx-conversation_xxx-generation_xxx-0-xxxx.png
```

访问图片时:

- 用户必须登录
- 普通用户只能访问自己的业务图片
- 管理员可以访问业务图片
- 删除会话时，会删除数据库记录和对应本地图片文件

## 8. Redis 是否需要

当前单机小范围上线不需要 Redis。

默认配置:

```toml
[storage]
backend = "current"
config_backend = "file"
```

此时:

- 配置从本地 `data/config.toml` 读取
- 核心业务数据存 SQLite
- 图片存在本地目录
- Job 使用 SQLite 持久化

只有在明确设置下面配置时才需要 Redis:

```toml
[storage]
backend = "redis"
config_backend = "redis"
```

或通过环境变量设置:

```text
STORAGE_BACKEND=redis
STORAGE_CONFIG_BACKEND=redis
```

如果配置成 Redis 但本地没有 Redis 服务，就会出现:

```text
redis: connection refused
```

当前建议:

- 小范围上线: 不启用 Redis
- 多进程、多机器部署: 再评估 Redis、PostgreSQL 或专用队列

## 9. API 接入配置

当前业务生图走管理员后台的 `账号管理 -> API 接入`。

工作台用户只选择平台:

- `gpt-image`
- `gemini-banana`

管理员在 API 接入里配置具体上游:

- 名称
- 平台
- Base URL
- API Key
- 默认模型
- 是否启用
- 是否默认

后端选择上游的优先级:

1. 数据库里的默认 API provider
2. `data/config.toml` 里的 `[api_access]`
3. 环境变量 `IMAGE_BASE_URL` / `IMAGE_API_KEY` / `IMAGE_MODEL`

推荐使用后台页面维护 API 接入，不推荐长期依赖环境变量。

### 9.1 gpt-image 示例

平台选择:

```text
gpt-image
```

Base URL 示例:

```text
http://127.0.0.1:8080
```

模型示例:

```text
gpt-image-2
```

### 9.2 gemini-banana 示例

平台选择:

```text
gemini-banana
```

模型可选:

```text
gemini-3.1-flash-image-preview
gemini-3-pro-image-preview
gemini-2.5-flash-image
gemini-2.5-flash-image-preview
```

## 10. 管理员与用户初始化

`ADMIN_USERNAME` / `ADMIN_PASSWORD` 和 `TEST_USERNAME` / `TEST_PASSWORD` 主要用于首次初始化数据库用户。

如果数据库里已经有同名用户，后续再改这些环境变量不会覆盖数据库里的密码。

首次启动时可用:

```bash
cd "/home/peng/Project/image/image-studio/web/backend"

API_ONLY=true \
SERVER_HOST=0.0.0.0 \
SERVER_PORT=7070 \
ADMIN_USERNAME=admin \
ADMIN_PASSWORD=admin123 \
TEST_USERNAME=test \
TEST_PASSWORD=test123 \
APP_SESSION_TTL_HOURS=24 \
go run .
```

初始化完成后，后续启动通常不需要再带这些账号密码环境变量。

## 11. 备份策略

当前最重要的是备份两类数据:

```text
/home/peng/Project/image/image-studio/web/backend/data/image-studio.db
/home/peng/Project/image/image-studio/web/backend/data/business-images
```

建议同时备份整个后端 `data` 目录:

```text
/home/peng/Project/image/image-studio/web/backend/data
```

因为里面还包含:

- `config.toml`
- `auths`
- `sync_state`
- `last-startup-error.txt`
- 旧图片兼容目录 `tmp/image`

备份前建议先暂停后端，避免 SQLite 正在写入。

## 12. 常见问题

### 12.1 `go: cannot find main module`

原因通常是在错误目录执行了 `go run .`。

正确目录:

```bash
cd "/home/peng/Project/image/image-studio/web/backend"
go run .
```

不要在下面目录运行后端:

```text
/home/peng/Project/image/image-studio/web
/home/peng/Project/image/image-studio/web/web
```

### 12.2 `connect ECONNREFUSED 127.0.0.1:7070`

原因是前端 Vite 正在运行，但后端 `7070` 没启动。

处理:

```bash
cd "/home/peng/Project/image/image-studio/web/backend"
API_ONLY=true SERVER_HOST=0.0.0.0 SERVER_PORT=7070 APP_SESSION_TTL_HOURS=24 go run .
```

### 12.3 `address already in use`

说明端口被占用。

查看占用:

```bash
lsof -i :7070
```

处理方式:

- 关闭旧后端进程
- 或修改 `SERVER_PORT`
- 或修改 `data/config.toml` 里的 `server.port`

### 12.4 `database is locked`

SQLite 单文件数据库同时写入时可能出现锁竞争。

当前已经做过 WAL、busy timeout 和短事务优化。小范围使用一般够用。

如果高频出现:

- 降低并发生成数
- 检查是否多个后端进程同时使用同一个 SQLite 文件
- 后续再迁移 PostgreSQL

### 12.5 Redis 连接失败

如果看到:

```text
redis: connection refused
```

先检查是否误配置:

```toml
storage.backend = "redis"
storage.config_backend = "redis"
```

当前建议改回:

```toml
backend = "current"
config_backend = "file"
```

## 13. 当前上线边界

当前版本适合:

- 单机部署
- 小范围内测
- 少量用户并发
- SQLite + 本地图片目录
- 管理员手动配置 API 接入

当前不建议直接用于:

- 多机器后端部署
- 大规模公开注册
- 高并发生图
- 需要严格任务恢复的生产场景

后续正式化方向:

- SQLite 迁移 PostgreSQL
- 图片目录迁移对象存储
- Job 队列迁移到持久化队列或 Postgres lease worker
- 增加更完整的备份与恢复流程
- 增加上游异步任务轮询能力
