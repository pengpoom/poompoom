# Image Studio 部署说明

更新时间: 2026-05-24

本文档记录当前部署、更新、备份和本地开发方式。生产部署推荐使用 Docker Compose，结构化业务数据保存在数据库服务中，图片文件和运行配置保存在部署目录的 `backend/data`。

## 1. 服务器要求

- Docker
- Docker Compose
- 可访问 GHCR 镜像仓库

服务器不需要安装 Go、Node.js 或 npm。

## 2. 快速部署

推荐使用部署脚本初始化目录：

```bash
mkdir -p image-studio
cd image-studio
curl -fsSL https://raw.githubusercontent.com/pengpoom/poomimage/main/deploy/docker-deploy.sh | bash
```

脚本会生成：

- `docker-compose.yml`
- `.env`
- `backend/data`
- `backups`

启动前先编辑 `.env`，至少确认：

- `ADMIN_PASSWORD`
- `TEST_PASSWORD`
- `POSTGRES_PASSWORD`
- `JOB_QUEUE_BACKEND`
- `IMAGE_STUDIO_DEPLOY_DIR`
- `DOCKER_CONFIG_DIR`
- `IMAGE_STUDIO_GITHUB_TOKEN`，私有仓库检测 tag / release 时需要
- （可选）启用对外图片 API 时需配 `EXTERNAL_API_ENABLED` / `EXTERNAL_API_BASE_URL` / `EXTERNAL_API_SIGNING_SECRET`，详见第 8 节

启动：

```bash
docker compose pull
docker compose up -d
docker compose logs -f studio
```

访问：

```text
http://服务器IP:7070/
```

## 3. 数据目录

默认持久化目录：

```text
backend/data
postgres-data volume
redis-data volume
```

`backend/data` 包含运行配置、图片文件和临时文件。数据库 volume 包含用户、会话、积分、job、provider、settings、tracker、图片会话和资产元数据。Redis volume 只保存短期队列信号，可通过数据库 queued job 扫描兜底恢复。

不要提交这些文件到 GitHub：

```text
.env
backend/data/config.toml
backend/data/business-images
backend/data/tmp/image
```

## 4. 更新服务

如果使用 `updater` 服务，管理员登录 Web 后可以点击左侧版本入口执行一键更新。更新过程会让 `updater` 在宿主机执行：

```bash
docker compose -p image-studio -f docker-compose.yml pull studio
docker compose -p image-studio -f docker-compose.yml up -d --remove-orphans studio
```

`COMPOSE_PROJECT_NAME` 必须和当前部署使用的 Compose 项目名一致。默认按 `image-studio` 部署即可；如果服务器已有旧部署，先用 `docker compose ls` 查看项目名，再把 `.env` 里的 `COMPOSE_PROJECT_NAME` 改成对应值。

`IMAGE_STUDIO_DEPLOY_DIR` 建议填写宿主机上的绝对部署目录，例如 `/root/image-studio`。这样 updater 在容器里执行 compose 时，相对挂载路径会按同一个宿主机目录解析。

也可以在服务器手动更新：

```bash
docker compose pull
docker compose up -d
docker compose logs -f studio
```

## 5. 备份

仓库提供数据目录备份脚本：

```bash
./scripts/backup-data.sh
```

默认备份 `backend/data` 到：

```text
backups/image-studio-data-YYYYmmdd-HHMMSS.tar.gz
```

数据库需要单独备份。示例：

```bash
docker compose exec postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > backups/image-studio-postgres-$(date +%Y%m%d-%H%M%S).sql
```

如果脚本不在仓库根目录，或者数据目录不同，可以显式指定：

```bash
IMAGE_STUDIO_DATA_DIR=/root/image-studio/backend/data \
IMAGE_STUDIO_BACKUP_DIR=/root/image-studio/backups \
./backup-data.sh
```

## 6. 恢复

恢复前先停止服务：

```bash
docker compose down
```

恢复 `backend/data`：

```bash
./scripts/restore-data.sh backups/image-studio-data-YYYYmmdd-HHMMSS.tar.gz
```

脚本会要求输入 `RESTORE`，并在覆盖前自动备份当前 `backend/data`。

恢复数据库后启动：

```bash
docker compose up -d
docker compose logs -f studio
```

## 7. 本地开发

启动本地依赖：

```bash
docker compose -p image-studio-postgres -f docker-compose.postgres.yml up -d postgres redis
```

启动后端：

```bash
cd backend
API_ONLY=true \
SERVER_HOST=0.0.0.0 \
SERVER_PORT=7070 \
CORS_ALLOWED_ORIGINS=http://localhost:5270,http://127.0.0.1:5270 \
DATABASE_DRIVER=postgres \
DATABASE_DSN=postgres://image_studio:image_studio@127.0.0.1:5432/image_studio?sslmode=disable \
JOB_QUEUE_BACKEND=redis \
REDIS_ADDR=127.0.0.1:6379 \
go run .
```

启动前端：

```bash
cd web
npm install
npm run dev
```

前端默认地址：

```text
http://localhost:5270/
```

## 8. 对外图片 API（/v1/images）配置

对外开发者分发 API（`/v1/images/*`，OpenAI 兼容）默认关闭。开启后，外部用户可以用业务 API Key + 你的域名作为 `base_url` 直接调用生图，无需登录 Web。

> 前提：运行的镜像需包含 `/v1` facade 代码。如果调用 `/v1/images/generations` 返回 404，说明镜像不含该功能，需要先用包含 facade 的版本重新构建/发布镜像。

### 8.1 开启与配置

在 `.env` 设置以下变量（或直接编辑挂载的 `backend/data/config.toml` 的 `[external_api]` 段），然后 `docker compose up -d studio` 重启：

```env
EXTERNAL_API_ENABLED=true
# 对外访问域名，终端用户会把它填进客户端的 base_url
EXTERNAL_API_BASE_URL=https://your-domain.com
# key 的 HMAC 根密钥，用 openssl rand -hex 32 生成
EXTERNAL_API_SIGNING_SECRET=<强随机值>
```

`enabled=false` 时整条对外 API 关闭，所有 `/v1/images/*` 请求返回 `503`。

### 8.2 signing_secret 是所有 key 的根，签发后不可更改

API Key 入库只保存 `HMAC-SHA256(signing_secret, key)`，不存明文。因此：

- **一旦签发过任何 key，`EXTERNAL_API_SIGNING_SECRET` 就不能再改**——改了之后所有已签发的 key 立即失效，调用返回 `401 invalid_api_key`，必须全部重新签发。
- 迁移服务器时，把同一个 `signing_secret` 一起带到新机器，老 key 才能继续用。
- 部署脚本 `deploy/docker-deploy.sh` 首次生成 `.env` 时会自动写入一个随机 `EXTERNAL_API_SIGNING_SECRET`；不要在已签发 key 之后再去改它。

### 8.3 签发 API Key

开启并重启后，管理员登录 Web 后台，在「API Keys」页为指定用户签发 `poom_live_…` key。明文只在创建时返回一次，请当场复制保存。

### 8.4 反向代理超时（nginx 等）

`POST /v1/images/generations` 默认是**同步**模式：服务端会把请求一直挂起，直到图片生成完成（最多 120 秒）才返回整图。如果服务前面挂了 nginx 之类的反向代理，它默认的 `proxy_read_timeout` 通常只有 60 秒，会在图片生成较慢时提前掐断连接，客户端收到 `504`（但图片其实可能已生成成功）。把对应 location 的超时调到比 120 秒更长：

```nginx
location / {
    proxy_pass http://127.0.0.1:7070;
    proxy_connect_timeout 75s;
    proxy_send_timeout    130s;
    proxy_read_timeout    130s;
}
```

```bash
nginx -t && systemctl reload nginx
```

如果客户端不希望长时间挂起，可在请求体加 `"async": true` 改用异步模式：立即返回 `202` + `status_url`，再用 `GET /v1/images/jobs/{id}` 轮询取图。CherryStudio、OpenAI SDK 这类标准客户端走默认同步即可，只要把反代超时调够。

## 9. 常见问题

### 8.1 `go: cannot find main module`

原因通常是在错误目录执行了 `go run .`。本地开发后端目录是：

```bash
cd backend
go run .
```

### 8.2 `connect ECONNREFUSED 127.0.0.1:7070`

前端 Vite 正在运行，但后端没有监听 `7070`。按“本地开发”里的后端命令启动即可。

### 8.3 `address already in use`

说明端口被占用。处理方式：

- 关闭旧后端进程
- 或修改 `SERVER_PORT`
- 或修改 `data/config.toml` 里的 `server.port`

### 8.4 Redis 连接失败

如果 `JOB_QUEUE_BACKEND=redis`，需要确保 Redis 服务已启动，且 `REDIS_ADDR` 指向正确地址。本地排查时可临时使用：

```text
JOB_QUEUE_BACKEND=local
```

### 8.5 数据库连接失败

检查 `.env` 或本地环境变量里的 `DATABASE_DRIVER`、`DATABASE_DSN`、数据库账号密码和容器健康状态：

```bash
docker compose ps
docker compose logs postgres
docker compose logs studio
```

## 10. 当前上线边界

当前版本适合：

- 单机 Docker Compose 部署
- 小范围公开或内测
- Redis 队列唤醒 + 数据库 job 兜底
- 本地图片目录持久化
- 管理员后台配置 API 接入
- 对外开发者分发 API（/v1/images，OpenAI 兼容，默认关闭）

后续正式化方向：

- 图片目录迁移对象存储
- 更完整的备份与恢复流程
- 增加上游异步任务轮询能力
