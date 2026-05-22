# Image Studio

Image Studio 是一个图片生成 Web 项目，包含 React 前端和 Go 后端。生产部署使用单个 Docker 镜像：前端会在构建时打包为静态资源，由后端统一托管。

默认服务端口是 `7000`，运行数据保存在部署目录的 `backend/data`。

## 快速部署

服务器只需要 Docker 和 Docker Compose，不需要在服务器上安装 Node.js 或 Go。

推荐使用部署脚本初始化目录：

```bash
mkdir -p poomimage
cd poomimage
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
- `IMAGE_STUDIO_DEPLOY_DIR`
- `DOCKER_CONFIG_DIR`
- `IMAGE_STUDIO_GITHUB_TOKEN`，私有仓库检测 tag / release 时需要

启动：

```bash
docker compose pull
docker compose up -d
docker compose logs -f studio
```

访问：

```text
http://服务器IP:7000/
```

如果当前仓库仍是私有仓库，直接 `curl raw.githubusercontent.com` 可能无法下载脚本。可以先 clone 仓库后执行：

```bash
git clone git@github.com:pengpoom/poomimage.git poomimage
cd poomimage
./deploy/docker-deploy.sh
```

也可以手动复制仓库里的 `docker-compose.yml` 和 `.env.example` 到服务器部署目录，并把 `.env.example` 改名为 `.env` 后再启动。

## 私有镜像登录

如果 GHCR 镜像不是公开镜像，服务器需要先登录：

```bash
docker login ghcr.io
```

`DOCKER_CONFIG_DIR` 必须指向执行 `docker login` 的用户 Docker 配置目录。root 部署通常是：

```env
DOCKER_CONFIG_DIR=/root/.docker
```

普通用户部署通常是：

```env
DOCKER_CONFIG_DIR=/home/用户名/.docker
```

updater 会把这个目录只读挂载到容器内，用于执行一键更新时 `docker compose pull studio`。

## 首次账号

首次启动时，如果数据库里还没有用户，服务会根据 `.env` 创建管理员和测试用户。

已有用户后，修改 `.env` 不会重置数据库里的账号密码。需要改密码时，请登录后台用户管理处理，或备份数据后再做数据库级维护。

## 更新服务

每次推送到 GitHub 后，GitHub Actions 会构建并发布新镜像到 GHCR。

如果使用上面的 `updater` 服务，管理员登录 Web 后可以点击左侧版本入口，执行一键更新。更新过程会让 `updater` 在宿主机执行：

```bash
docker compose -p image-studio -f docker-compose.yml pull studio
docker compose -p image-studio -f docker-compose.yml up -d --remove-orphans studio
```

`studio` 容器会短暂重建，前端会轮询 `/health`，服务恢复后自动刷新页面。

`COMPOSE_PROJECT_NAME` 必须和当前部署使用的 Compose 项目名一致。默认按上面的 `image-studio` 部署即可；如果服务器已有旧部署，先用 `docker compose ls` 查看项目名，再把 `.env` 里的 `COMPOSE_PROJECT_NAME` 改成对应值。

`IMAGE_STUDIO_DEPLOY_DIR` 建议填写宿主机上的绝对部署目录，例如 `/root/image-studio`。这样 updater 在容器里执行 compose 时，相对挂载路径会按同一个宿主机目录解析，不会误建 `/deploy/backend/data` 这类新数据目录。

版本检查会读取 GitHub 最新 tag，并尽量补充对应 release 信息。私有仓库需要设置 `IMAGE_STUDIO_GITHUB_TOKEN`，否则 GitHub API 会返回未授权。

也可以在服务器手动更新：

```bash
docker compose pull
docker compose up -d
docker compose logs -f studio
```

如果部署目录是完整仓库，也可以使用：

```bash
./scripts/docker-update.sh
```

## 数据目录

默认持久化目录：

```text
backend/data
```

这里包含运行配置、SQLite 数据库、用户数据、图片历史和生成图片文件。服务器迁移或重装时，优先备份这个目录。

注意不要提交这些文件到 GitHub：

```text
.env
backend/data/config.toml
backend/data/image-studio.db
backend/data/business-images
backend/data/tmp/image
```

## 备份

仓库提供 Bash 脚本：

```bash
./scripts/backup-data.sh
```

默认备份 `backend/data` 到：

```text
backups/image-studio-data-YYYYmmdd-HHMMSS.tar.gz
```

如果服务器部署目录只有 `docker-compose.yml`，可以把 `scripts/backup-data.sh` 和 `scripts/restore-data.sh` 复制到部署目录使用。

如果脚本不在仓库根目录，或者数据目录不同，可以显式指定：

```bash
IMAGE_STUDIO_DATA_DIR=/root/image-studio/backend/data \
IMAGE_STUDIO_BACKUP_DIR=/root/image-studio/backups \
./backup-data.sh
```

## 恢复

恢复前先停止服务：

```bash
docker compose down
```

执行恢复：

```bash
./scripts/restore-data.sh backups/image-studio-data-YYYYmmdd-HHMMSS.tar.gz
```

脚本会要求输入 `RESTORE`，并在覆盖前自动备份当前 `backend/data`。

恢复后启动：

```bash
docker compose up -d
docker compose logs -f studio
```

## 本地开发

后端：

```bash
cd backend
API_ONLY=true \
SERVER_HOST=0.0.0.0 \
SERVER_PORT=7070 \
CORS_ALLOWED_ORIGINS=http://localhost:5270,http://127.0.0.1:5270 \
go run .
```

前端：

```bash
cd web
npm install
npm run dev
```

前端默认地址：

```text
http://localhost:5270/
```

## 常用检查

后端测试：

```bash
cd backend
go test ./api ./internal/middleware
```

前端测试和构建：

```bash
cd web
npm run test
npm run build
```
