# Image Studio 接口文档

更新时间：2026-05-17

本文档以当前后端实际挂载路由为准，主要来源是 `web/backend/api/server.go` 的 `Server.Handler()`。前端代码里仍保留的旧账号接口如果没有在当前路由表挂载，会单独放在“历史遗留 / 未挂载接口”说明中。

## 基础约定

默认后端地址：

```text
http://127.0.0.1:7070
```

默认前端开发地址：

```text
http://localhost:5270
```

认证方式分为三类：

| 类型 | 用途 | 说明 |
| --- | --- | --- |
| 公开接口 | 登录、版本、健康检查 | 不需要登录 |
| UI 登录态 | 前端工作台、个人信息、业务生图 | `POST /auth/login` 后返回 `token`；普通 `/api/*` 路由主要使用 `Authorization: Bearer <token>` |
| 管理员登录态 | 管理后台接口 | 需要 UI Bearer token，且登录用户 `role=admin` |
| 图片文件登录态 | 业务图片访问 | `/v1/files/image/business-*` 支持 UI Bearer token 或 `image_studio_session` Cookie |
| 兼容 API Key | `/v1/*` 兼容接口 | `Authorization: Bearer <APP_API_KEY>`；也兼容 UI Bearer token |

统一错误格式优先使用：

```json
{
  "error": {
    "message": "错误说明",
    "type": "invalid_request_error",
    "code": "optional_error_code"
  }
}
```

少量历史接口仍可能返回：

```json
{
  "error": "错误说明"
}
```

常见状态码：

| 状态码 | 含义 |
| --- | --- |
| 200 | 成功 |
| 201 | 创建成功 |
| 202 | 已接受，后台继续处理 |
| 400 | 请求体或参数错误 |
| 401 | 未登录或 token 无效 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 资源冲突，例如用户名重复、任务取消 |
| 429 | 生图队列已满 |
| 502 | 上游 API 调用失败 |
| 504 | 生图排队超时 |

## 公开接口

### POST `/auth/login`

账号密码登录。登录成功后会返回 token，并设置 `image_studio_session` Cookie。

请求：

```json
{
  "username": "admin",
  "password": "admin123"
}
```

响应：

```json
{
  "ok": true,
  "token": "sess_xxx",
  "role": "admin",
  "username": "admin",
  "userId": "dev_admin",
  "expiresAt": "2026-05-18T15:00:00+08:00",
  "version": "v1.2.10"
}
```

说明：

- `role=admin` 进入管理后台。
- `role=user` 进入普通用户工作区。
- 前端普通 API 请求使用返回的 `token` 放入 `Authorization: Bearer <token>`。
- `image_studio_session` Cookie 主要用于新标签页直接打开业务图片 URL，以及退出登录时兜底识别 session。
- 首次启动时，默认管理员和测试用户会从环境变量初始化到数据库；用户已存在时，后续改环境变量不会覆盖数据库密码。

### POST `/auth/logout`

退出登录，撤销当前 session，并清除 Cookie。

响应：

```json
{
  "ok": true
}
```

### GET `/version`

获取后端版本信息。

响应：

```json
{
  "version": "v1.2.10",
  "commit": "",
  "buildTime": ""
}
```

### GET `/health`

健康检查。

响应：

```json
{
  "status": "ok"
}
```

## 业务生图接口

### POST `/api/image/generate`

当前图片工作台使用的主生图提交接口。需要 UI 登录态。

当前接口是异步 job 模式：请求成功后先返回 `202 Accepted + jobId`，真正生图由后台 runner 继续执行。前端应通过 `GET /api/business/jobs/{id}` 或刷新对应业务会话读取最终结果。

后端会把提交请求保存到 `business_image_jobs.payload_json`。新提交不会在请求线程里直接同步等待生图，而是写入 `queued` job 后唤醒后台 worker，由 worker 从数据库 claim 后执行。如果后端在 job 仍处于 `queued` 时重启，启动后的 worker 会扫描并恢复执行这些 queued job。已经进入 `running` 的 job 仍按超时兜底处理，不会盲目重复请求上游。

请求：

```json
{
  "prompt": "生成一张雨伞的图片",
  "model": "gpt-image-2",
  "n": 1,
  "size": "1024x1024",
  "quality": "high",
  "response_format": "url",
  "platform": "gpt-image",
  "jobId": "job_xxx",
  "conversationId": "conv_xxx",
  "turnId": "turn_xxx",
  "title": "生成一张雨伞的图片"
}
```

字段说明：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `prompt` | 是 | 生图提示词 |
| `model` | 否 | 模型名；为空时按平台默认模型 |
| `n` | 否 | 图片数量，默认 1 |
| `size` | 否 | 图片尺寸，如 `1024x1024` |
| `quality` | 否 | 质量，如 `low`、`high` |
| `response_format` | 否 | 当前业务侧通常使用 `url` |
| `platform` | 否 | `gpt-image` 或 `gemini-banana` |
| `jobId` | 否 | 前端生成的任务 ID；当前阶段通常与 generation ID 共用 |
| `conversationId` | 否 | 会话 ID |
| `turnId` | 否 | 本轮生成 ID |
| `title` | 否 | 会话标题 |

成功提交响应：`202 Accepted`

```json
{
  "jobId": "job_xxx",
  "conversationId": "conv_xxx",
  "turnId": "turn_xxx",
  "status": "queued",
  "job": {
    "id": "job_xxx",
    "userId": "user_xxx",
    "conversationId": "conv_xxx",
    "generationId": "job_xxx",
    "turnId": "turn_xxx",
    "platform": "gpt-image",
    "model": "gpt-image-2",
    "prompt": "生成一张雨伞的图片",
    "requestedCount": 1,
    "status": "queued",
    "stage": "queued"
  }
}
```

处理流程：

1. 校验登录用户。
2. 校验基础请求和 API provider 配置。
3. 创建 `business_image_jobs` queued 记录。
4. 保存原始请求 payload，用于后端重启后恢复 queued job。
5. 写入 queued generation 占位。
6. 立即返回 `202 Accepted`。
7. 后台 runner 进入 admission 准入队列。
8. 扣点 reserve。
9. 请求上游生图 API；对明确的临时上游错误最多重试 1 次。
10. 将图片持久化到本地图片目录。
11. 写入会话、生成记录、资产表、使用记录、tracker、job 状态。
12. 失败或取消时按系统设置决定是否退点。

常见错误：

| 状态码 | code | message |
| --- | --- | --- |
| 400 | `invalid_request` | `prompt is required` 或请求体错误 |
| 500 | `provider_not_configured` | API 接入未配置 |

说明：点数不足、队列满、上游失败、取消等发生在后台 runner 阶段时，不再作为本次 HTTP 提交响应返回，而是写入 job / generation 状态。前端需要查询 job 或会话详情展示最终结果。

## 业务会话接口

这些接口读取的是业务数据库中的图片会话、生成记录和资产记录。需要 UI 登录态，并按当前登录用户的 `user_id` 限定数据范围。

### GET `/api/business/image/conversations`

获取当前用户的图片会话列表。

查询参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `limit` | 50 | 最大返回数量 |

响应：

```json
{
  "userId": "user_xxx",
  "items": [
    {
      "id": "conv_xxx",
      "user_id": "user_xxx",
      "title": "生成一张雨伞的图片",
      "created_at": "2026-05-17T08:00:00Z",
      "updated_at": "2026-05-17T08:01:00Z"
    }
  ]
}
```

### GET `/api/business/image/conversations/{id}`

获取当前用户某个会话详情和生成记录。

查询参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `limit` | 100 | 最大返回生成记录数量 |

响应：

```json
{
  "item": {
    "conversation": {
      "id": "conv_xxx",
      "title": "生成一张雨伞的图片"
    },
    "generations": [
      {
        "id": "turn_xxx",
        "conversation_id": "conv_xxx",
        "prompt": "生成一张雨伞的图片",
        "model": "gpt-image-2",
        "size": "1024x1024",
        "quality": "high",
        "count": 1,
        "status": "succeeded",
        "response": {
          "data": [
            {
              "url": "/v1/files/image/business-xxx.png"
            }
          ]
        }
      }
    ]
  }
}
```

### PATCH `/api/business/image/conversations/{id}`

重命名当前用户的单个业务图片会话。

请求：

```json
{
  "title": "新的会话名"
}
```

说明：

- `title` 会自动去除首尾空格。
- `title` 不能为空，最长 80 个字符。
- 不允许重命名其他用户的会话。

响应：

```json
{
  "item": {
    "id": "conv_xxx",
    "user_id": "user_xxx",
    "title": "新的会话名",
    "created_at": "2026-05-17T08:00:00Z",
    "updated_at": "2026-05-17T08:02:00Z"
  }
}
```

### DELETE `/api/business/image/conversations/{id}`

删除当前用户的单个会话。

说明：

- 删除会话记录。
- 删除会话下的生成记录。
- 删除资产表中对应记录。
- 删除不再被其他记录引用的本地业务图片文件。
- 不允许删除其他用户的会话。

响应：

```json
{
  "ok": true,
  "userId": "user_xxx",
  "result": {
    "conversations": 1,
    "generations": 2,
    "assets": 2
  },
  "deletedFiles": 2
}
```

### DELETE `/api/business/image/conversations`

清空当前用户的所有业务图片会话。

响应结构同单个删除接口。

## 业务任务接口

任务表用于保存生图请求的中间状态，让用户切页或刷新后仍能看到 `queued`、`running`、`succeeded`、`failed` 等状态。当前阶段生成接口已经是后台 job runner 模式：提交接口先返回 `jobId`，后台继续执行真正生图。

恢复策略：

- 后端启动时先扫描 `queued` job。
- worker 会 claim 这些 job，并用 `payload_json` 恢复原始请求继续执行。
- 新提交 job 也通过同一个数据库 worker claim 流程执行。
- claim 会使用短 lease，避免 worker 被重复唤醒时同一个 `queued` job 被重复执行。
- 后端会定时执行 stale reconcile，收敛长期停留在 `queued`、`running`、`cancel_requested` 的 job。
- 没有 `payload_json` 的旧 queued job，会用 job 表里的 prompt、model、size、quality、count、platform 重建最小请求。
- `running` / `cancel_requested` 不会重放请求，仍交给 stale 兜底自动失败、取消和退款。

任务状态：

| 状态 | 说明 |
| --- | --- |
| `queued` | 已收到请求，等待进入准入队列或等待执行 |
| `running` | 已开始调用上游或持久化 |
| `cancel_requested` | 用户请求取消 |
| `cancelled` | 已取消 |
| `succeeded` | 已成功 |
| `failed` | 已失败 |

### GET `/api/business/jobs`

获取当前用户的图片任务列表。

查询参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `conversationId` | 空 | 限定某个会话 |
| `limit` | 50 | 返回数量 |

响应：

```json
{
  "items": [
    {
      "id": "job_xxx",
      "userId": "user_xxx",
      "conversationId": "conv_xxx",
      "generationId": "job_xxx",
      "turnId": "turn_xxx",
      "platform": "gpt-image",
      "providerId": "provider_xxx",
      "providerName": "默认 GPT 接入",
      "model": "gpt-image-2",
      "prompt": "生成一张雨伞的图片",
      "size": "1024x1024",
      "quality": "high",
      "requestedCount": 1,
      "actualCount": 1,
      "status": "succeeded",
      "stage": "done",
      "queueWaitMs": 15,
      "upstreamDurationMs": 12000,
      "persistDurationMs": 40,
      "totalDurationMs": 12100,
      "storageBytes": 1339319,
      "creditReserved": 1,
      "creditRefunded": 0,
      "createdAt": "2026-05-17T08:00:00Z",
      "updatedAt": "2026-05-17T08:00:12Z"
    }
  ]
}
```

### GET `/api/business/jobs/{id}`

获取当前用户某个任务详情。

响应：

```json
{
  "item": {
    "id": "job_xxx",
    "status": "running",
    "stage": "provider_request"
  }
}
```

### POST `/api/business/jobs/{id}/cancel`

请求取消当前用户某个任务。

取消会同步更新 job 状态；如果对应 generation 仍处于 `queued`、`running` 或 `cancel_requested`，也会同步标记为 `cancelled`，便于刷新会话后前端立即显示取消状态。

响应：

```json
{
  "item": {
    "id": "job_xxx",
    "status": "cancel_requested"
  },
  "activeCancelled": true
}
```

说明：

- `activeCancelled=true` 表示命中了当前进程内正在执行的任务上下文。
- 上游请求是否能被立即中断，取决于当前请求阶段和上游连接是否响应 context 取消。

## 图片文件接口

### GET `/v1/files/image/{filename}`

读取本地持久化图片文件。

业务图片文件名以 `business-` 开头，需要 UI 登录态：

- 普通用户只能访问自己资产表中引用的图片。
- 管理员可以访问所有业务图片。
- 业务图片响应头使用私有缓存：`Cache-Control: private, max-age=0, must-revalidate`。

非业务旧缓存图片不要求业务资产鉴权，并使用长期公共缓存。

示例：

```text
GET /v1/files/image/business-user_xxx-conv_xxx-turn_xxx-0-abcd.png
```

## 个人用户接口

### GET `/api/business/me`

获取当前登录用户信息和点数。

响应：

```json
{
  "user": {
    "id": "user_xxx",
    "username": "test",
    "role": "user",
    "status": "active",
    "created_at": "2026-05-17T08:00:00Z",
    "updated_at": "2026-05-17T08:00:00Z"
  },
  "credit": {
    "user_id": "user_xxx",
    "balance": 20,
    "spent": 0,
    "updated_at": "2026-05-17T08:00:00Z"
  }
}
```

### PATCH `/api/business/me/password`

当前用户修改自己的密码。

请求：

```json
{
  "currentPassword": "old-pass",
  "newPassword": "new-pass"
}
```

响应：

```json
{
  "user": {
    "id": "user_xxx",
    "username": "test",
    "role": "user",
    "status": "active"
  }
}
```

### GET `/api/business/credit`

获取当前用户点数余额。

响应：

```json
{
  "user_id": "user_xxx",
  "balance": 18,
  "spent": 2,
  "updated_at": "2026-05-17T08:10:00Z"
}
```

### GET `/api/business/usage`

获取当前用户自己的使用记录。

查询参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `page` | 1 | 页码 |
| `pageSize` | 20 | 每页数量，最大 100 |
| `status` | 空 | `succeeded`、`failed` 等 |
| `model` | 空 | 模型名 |
| `from` | 空 | ISO 时间 |
| `to` | 空 | ISO 时间 |
| `timeRange` | 空 | 快捷时间范围 |
| `timezone` | 本地时区 | 例如 `Asia/Shanghai` |

`timeRange` 支持：

```text
today, 24h, last24h, yesterday, last7, 7d, last14, 14d, last30, 30d, month, this_month, last_month, custom
```

响应：

```json
{
  "items": [
    {
      "id": "usage_xxx",
      "user_id": "user_xxx",
      "conversation_id": "conv_xxx",
      "generation_id": "turn_xxx",
      "turn_id": "turn_xxx",
      "prompt": "生成一张雨伞的图片",
      "model": "gpt-image-2",
      "size": "1024x1024",
      "quality": "high",
      "count": 1,
      "status": "succeeded",
      "duration_ms": 12100,
      "credit_delta": -1,
      "credits_used": 1,
      "created_at": "2026-05-17T08:00:00Z",
      "finished_at": "2026-05-17T08:00:12Z"
    }
  ],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

## 管理员：仪表盘和用量

### GET `/api/business/admin/dashboard`

管理员总览仪表盘。

支持使用记录筛选参数：`status`、`model`、`from`、`to`、`timeRange`、`timezone`。

响应：

```json
{
  "summary": {
    "userCount": 2,
    "adminCount": 1,
    "activeUserCount": 2,
    "disabledUserCount": 0,
    "generationCount": 10,
    "successCount": 9,
    "failedCount": 1,
    "imageCount": 9,
    "storageBytes": 12000000,
    "conversationCount": 5,
    "creditBalance": 30,
    "creditSpent": 10,
    "lastGeneratedAt": "2026-05-17T08:00:00Z",
    "recentUsageCount": 10,
    "recentSuccessCount": 9,
    "recentFailedCount": 1,
    "recentImageCount": 9,
    "recentCreditsUsed": 10
  },
  "recent": [],
  "topUsers": [],
  "modelUsage": [],
  "page": {
    "page": 1,
    "pageSize": 10,
    "total": 10
  }
}
```

### GET `/api/business/admin/usage`

管理员查看所有用户使用记录。

查询参数同 `/api/business/usage`，额外支持：

| 参数 | 说明 |
| --- | --- |
| `userId` | 限定某个用户 |

响应中每条记录额外带 `username`。

### GET `/api/business/admin/jobs`

管理员查看所有用户的图片 job 列表。用于运维监控里的 Job 明细，不开放给普通用户。

运维监控前端会展示状态、用户、平台、模型、provider、提示词、job/generation/conversation ID、队列等待、上游耗时、持久化耗时、实际图片数、存储大小、扣点、退款、时间线和错误信息。

查询参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `page` | 1 | 页码 |
| `pageSize` | 20 | 每页数量，最大 100 |
| `userId` | 空 | 限定某个用户 |
| `status` | 空 | 限定状态：`queued`、`running`、`cancel_requested`、`cancelled`、`succeeded`、`failed` |
| `platform` | 空 | 限定平台：`gpt-image`、`gemini-banana` |
| `timeRange` | 空 | 快捷时间范围，逻辑同使用记录筛选 |
| `from` | 空 | 自定义开始时间 |
| `to` | 空 | 自定义结束时间 |
| `timezone` | 空 | 时间范围解析时区 |

响应：

```json
{
  "items": [
    {
      "id": "job_xxx",
      "userId": "user_xxx",
      "conversationId": "conv_xxx",
      "generationId": "job_xxx",
      "platform": "gpt-image",
      "model": "gpt-image-2",
      "prompt": "生成一张雨伞的图片",
      "status": "succeeded",
      "stage": "done",
      "queueWaitMs": 15,
      "upstreamDurationMs": 12000,
      "persistDurationMs": 40,
      "totalDurationMs": 12100,
      "storageBytes": 1339319,
      "creditReserved": 1,
      "creditRefunded": 0,
      "createdAt": "2026-05-17T08:00:00Z",
      "updatedAt": "2026-05-17T08:00:12Z"
    }
  ],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

## 管理员：用户管理

### GET `/api/business/users`

获取业务用户列表。

响应：

```json
{
  "items": [
    {
      "id": "user_xxx",
      "username": "test",
      "role": "user",
      "status": "active",
      "usage": {
        "generation_count": 3,
        "success_count": 3,
        "failed_count": 0,
        "image_count": 3,
        "storage_bytes": 6000000
      },
      "credit": {
        "balance": 17,
        "spent": 3
      }
    }
  ]
}
```

### POST `/api/business/users`

创建业务用户。

请求：

```json
{
  "username": "new-user",
  "password": "new-pass",
  "role": "user"
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `username` | 用户名，唯一 |
| `password` | 密码 |
| `role` | `admin` 或 `user` |

响应：`201`

```json
{
  "item": {
    "id": "user_xxx",
    "username": "new-user",
    "role": "user",
    "status": "active"
  }
}
```

说明：创建用户时，如果系统设置里配置了 `user.defaultCredits`，会自动初始化点数。

### GET `/api/business/users/{id}`

查看单个用户详情。

查询参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `usagePage` | 1 | 使用记录页码 |
| `usagePageSize` | 20 | 使用记录每页数量 |
| `assetsPage` | 1 | 图片资产页码 |
| `assetsPageSize` | 20 | 图片资产每页数量 |
| `ledgerPage` | 1 | 点数流水页码 |
| `ledgerPageSize` | 20 | 点数流水每页数量 |
| `status` | 空 | 使用记录状态筛选 |
| `model` | 空 | 使用记录模型筛选 |
| `from` | 空 | 使用记录开始时间 |
| `to` | 空 | 使用记录结束时间 |
| `timeRange` | 空 | 快捷时间范围 |
| `timezone` | 本地时区 | 时区 |

响应：

```json
{
  "user": {},
  "usage": {},
  "credit": {},
  "recentUsage": [],
  "recentUsagePage": {},
  "recentAssets": [],
  "recentAssetsPage": {},
  "recentCreditLedger": [],
  "recentLedgerPage": {},
  "conversationCount": 3
}
```

### PATCH `/api/business/users/{id}/status`

启用或禁用用户。

请求：

```json
{
  "status": "disabled"
}
```

响应：

```json
{
  "item": {
    "id": "user_xxx",
    "status": "disabled"
  }
}
```

限制：不能禁用当前登录的管理员自己。

### PATCH `/api/business/users/{id}/password`

管理员重置用户密码。

请求：

```json
{
  "password": "new-pass"
}
```

响应：

```json
{
  "item": {
    "id": "user_xxx",
    "username": "test"
  }
}
```

### PUT `/api/business/users/{id}/credit`

管理员设置用户点数余额。

请求：

```json
{
  "balance": 100,
  "reason": "admin_adjustment"
}
```

响应：

```json
{
  "credit": {
    "user_id": "user_xxx",
    "balance": 100,
    "spent": 3
  },
  "ledger": {
    "id": "ledger_xxx",
    "user_id": "user_xxx",
    "delta": 83,
    "balance_after": 100,
    "reason": "admin_adjustment",
    "created_at": "2026-05-17T08:00:00Z"
  }
}
```

## 管理员：API 接入

API 接入用于配置不同平台的上游生图服务。工作台用户只选择平台，例如 `gpt-image` 或 `gemini-banana`，业务后端根据平台选择启用且默认的 provider。

支持平台：

| 平台 | 后端适配器 | 默认模型 |
| --- | --- | --- |
| `gpt-image` | OpenAI-compatible image API | `gpt-image-2` |
| `gemini-banana` | Gemini Banana adapter | `gemini-2.5-flash-image` |

Gemini 下拉模型当前包含：

```text
gemini-3.1-flash-image-preview
gemini-3-pro-image-preview
gemini-2.5-flash-image
gemini-2.5-flash-image-preview
```

Provider 对象：

```json
{
  "id": "provider_xxx",
  "name": "默认 GPT 接入",
  "platform": "gpt-image",
  "baseUrl": "http://127.0.0.1:8080",
  "apiKey": "sk-...",
  "defaultModel": "gpt-image-2",
  "enabled": true,
  "isDefault": true,
  "createdAt": "2026-05-17T08:00:00Z",
  "updatedAt": "2026-05-17T08:00:00Z"
}
```

### GET `/api/business/api-providers`

获取 API 接入列表。

响应：

```json
{
  "items": []
}
```

### POST `/api/business/api-providers`

新增 API 接入。

请求：

```json
{
  "name": "Gemini 接入",
  "platform": "gemini-banana",
  "baseUrl": "https://example.com",
  "apiKey": "sk-xxx",
  "defaultModel": "gemini-2.5-flash-image",
  "enabled": true,
  "isDefault": true
}
```

响应：`201`

```json
{
  "item": {}
}
```

### PUT `/api/business/api-providers/{id}`

编辑 API 接入。

请求体同新增接口。`apiKey` 为空时按当前实现会保存为空，因此编辑时前端应传入要保留或替换的值。

### POST `/api/business/api-providers/{id}/default`

设置某个 API 接入为默认。

响应：

```json
{
  "item": {
    "id": "provider_xxx",
    "isDefault": true
  }
}
```

### POST `/api/business/api-providers/{id}/test`

测试某个 API 接入是否能完成最小生图请求。

成功响应：

```json
{
  "ok": true,
  "message": "provider test succeeded",
  "durationMs": 12000,
  "imageCount": 1
}
```

失败响应：`502`

```json
{
  "ok": false,
  "message": "上游错误说明",
  "code": "provider_error",
  "durationMs": 3000,
  "imageCount": 0
}
```

### DELETE `/api/business/api-providers/{id}`

删除 API 接入。

响应：

```json
{
  "ok": true
}
```

## 管理员：系统设置

### GET `/api/business/system-settings`

获取业务系统设置和运行时路径信息。

响应：

```json
{
  "settings": {
    "site": {
      "name": "ImageStudio",
      "subtitle": "图片生成工作台",
      "logoUrl": "",
      "contactInfo": ""
    },
    "user": {
      "defaultRole": "user",
      "defaultCredits": 20,
      "registration": false
    },
    "generation": {
      "defaultPlatform": "gpt-image",
      "defaultQuality": "high",
      "defaultSize": "1024x1024",
      "defaultCount": 1,
      "maxCount": 8
    },
    "billing": {
      "gptImageCost": 1,
      "geminiBananaCost": 1,
      "refundOnFailure": true,
      "refundPartialCount": true
    },
    "runtime": {
      "maxImageConcurrency": 8,
      "imageQueueLimit": 32,
      "imageQueueTimeoutSeconds": 20
    },
    "security": {
      "imageFileAuthRequired": true
    }
  },
  "runtime": {
    "sqlitePath": "data/image-studio.db",
    "imageDir": "data/business-images",
    "imageFileAuthRequired": true,
    "legacyConfigWritable": true,
    "businessSettingsTableName": "business_system_settings"
  }
}
```

### PUT `/api/business/system-settings`

更新业务系统设置。

请求：

```json
{
  "settings": {
    "site": {
      "name": "ImageStudio",
      "subtitle": "图片生成工作台"
    },
    "user": {
      "defaultRole": "user",
      "defaultCredits": 20,
      "registration": false
    },
    "generation": {
      "defaultPlatform": "gpt-image",
      "defaultQuality": "high",
      "defaultSize": "1024x1024",
      "defaultCount": 1,
      "maxCount": 8
    },
    "billing": {
      "gptImageCost": 1,
      "geminiBananaCost": 1,
      "refundOnFailure": true,
      "refundPartialCount": true
    },
    "runtime": {
      "maxImageConcurrency": 8,
      "imageQueueLimit": 32,
      "imageQueueTimeoutSeconds": 20
    },
    "security": {
      "imageFileAuthRequired": true
    }
  }
}
```

响应同获取接口。

说明：

- `runtime.maxImageConcurrency`：最大同时生图数量。
- `runtime.imageQueueLimit`：最大排队数量。
- `runtime.imageQueueTimeoutSeconds`：排队最长等待时间。
- 更新后会立即应用到当前进程中的 admission 准入队列配置。

## 管理员：存储检查

### GET `/api/business/storage/report`

检查业务图片资产表和磁盘文件的一致性。

响应：

```json
{
  "summary": {
    "assetFiles": 4,
    "diskFiles": 5,
    "referencedFiles": 4,
    "missingFiles": 0,
    "orphanFiles": 1,
    "legacyReferencedFiles": 0,
    "brokenAssets": 0,
    "assetBytes": 6000000,
    "diskBytes": 7000000,
    "orphanBytes": 1000000
  },
  "missingFiles": [],
  "orphanFiles": [],
  "legacyReferencedFiles": [],
  "brokenAssets": [],
  "directories": [
    "/home/peng/Project/image/image-studio/web/backend/data/business-images",
    "/home/peng/Project/image/image-studio/web/backend/data/tmp/image"
  ]
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `missingFiles` | 资产表有记录，但磁盘文件不存在 |
| `orphanFiles` | 磁盘有 `business-*` 文件，但数据库没人引用 |
| `legacyReferencedFiles` | 旧生成记录里有图片 URL，但没有资产表记录 |
| `brokenAssets` | 资产表指向的 generation 或 user 找不到 |

### POST `/api/business/storage/backfill-assets`

尝试把旧引用补录进业务资产表，然后返回新的存储检查报告。

响应：

```json
{
  "result": {
    "inserted": 1,
    "skipped": 0,
    "missing": 0
  },
  "report": {}
}
```

## 管理员：运行监控

### GET `/api/runtime/status`

获取当前进程运行状态，主要是 admission 准入队列和最近错误。

响应：

```json
{
  "timestamp": "2026-05-17T08:00:00Z",
  "admission": {
    "maxConcurrency": 8,
    "queueLimit": 32,
    "queueTimeoutMs": 20000,
    "inflight": 0,
    "queued": 0
  },
  "recent": {
    "windowSeconds": 600,
    "failureCount": 0,
    "lastError": "",
    "lastErrorCode": "",
    "lastErrorAt": "",
    "lastErrorAccount": ""
  }
}
```

### GET `/api/business/tracker/summary`

获取业务生图 tracker 汇总。用于运维监控里展示最近窗口内的成功、失败、耗时和阶段分布。

查询参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `windowSeconds` | 600 | 统计窗口秒数 |

响应结构由 tracker store 返回，主要包含最近窗口内的任务数量、状态分布、阶段分布、失败原因和耗时数据。

### POST `/api/tools/admission-stress`

管理员压测 admission 准入队列。仅用于开发和运维诊断。

请求：

```json
{
  "total": 120,
  "workers": 20,
  "holdMs": 100,
  "timeoutSeconds": 10
}
```

响应：

```json
{
  "startedAt": "2026-05-17T08:00:00Z",
  "finishedAt": "2026-05-17T08:00:01Z",
  "durationMs": 1000,
  "timedOut": false,
  "input": {},
  "admission": {},
  "counters": {
    "submitted": 120,
    "attempted": 120,
    "admitted": 120,
    "queueFull": 0,
    "queueTimeout": 0,
    "canceled": 0,
    "otherErrors": 0
  },
  "queueWait": {
    "samples": 120,
    "avgMs": 10,
    "p50Ms": 8,
    "p95Ms": 30,
    "maxMs": 50
  }
}
```

## 管理员：诊断、配置和同步

这些接口偏底层或历史兼容，当前主要用于系统设置、诊断导出和迁移排查。

### GET `/api/startup/check`

启动检查。检查后端服务、API 接入、存储路径等状态。

响应：

```json
{
  "startedAt": "2026-05-17T08:00:00Z",
  "finishedAt": "2026-05-17T08:00:00Z",
  "overall": "pass",
  "passCount": 3,
  "warnCount": 0,
  "failCount": 0,
  "checks": [
    {
      "key": "api_providers",
      "label": "API 接入",
      "status": "pass",
      "detail": "API 接入总数 2，启用 2",
      "durationMs": 1
    }
  ],
  "summaryText": "..."
}
```

### GET `/api/diagnostics/export`

导出诊断 JSON 文件，包含版本、启动检查、运行时状态、脱敏配置、请求日志。

响应：JSON 文件下载。

当前文件名仍是历史命名：

```text
image-studio-diagnostics-YYYYMMDD-HHMMSS.json
```

### GET `/api/config`

获取完整底层配置。管理员接口。

响应字段包括：

```text
app, server, chatgpt, accounts, storage, sync, proxy, apiAccess, newapi, sub2api, log, paths
```

说明：当前主要业务配置已迁移到 `/api/business/system-settings` 和 `/api/business/api-providers`，不建议新页面继续依赖这个低层配置接口。

### GET `/api/config/defaults`

获取默认底层配置。

### PUT `/api/config`

更新底层配置。

说明：

- 这是低层配置写入接口。
- 可能影响存储路径、旧兼容模式、代理、同步等行为。
- 当前业务后台新增功能优先使用系统设置和 API 接入接口。

### POST `/api/proxy/test`

测试代理地址。

请求：

```json
{
  "url": "http://127.0.0.1:7890"
}
```

### POST `/api/integration/test`

测试 NewAPI 或 Sub2API 配置。

请求：

```json
{
  "source": "newapi",
  "newapi": {}
}
```

或：

```json
{
  "source": "sub2api",
  "sub2api": {}
}
```

### POST `/api/integration/newapi/token`

发现 NewAPI token。

请求：

```json
{
  "newapi": {}
}
```

### POST `/api/integration/sub2api/groups`

读取 Sub2API 分组。

请求：

```json
{
  "sub2api": {}
}
```

### GET `/api/sync/status`

获取旧账号同步状态。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `source` | 同步源，如 `cpa` |
| `progress_only` | 只返回进度 |

### POST `/api/sync/run`

触发旧账号同步。

请求：

```json
{
  "direction": "pull",
  "source": "cpa"
}
```

## OpenAI 兼容接口

这些接口服务于 `/v1/*` 兼容调用，认证使用 `Authorization: Bearer <APP_API_KEY>`，也允许已登录 UI session。它们属于兼容层，不是当前 Image Studio 工作台主链路。

`/v1/images/generations` 和 `/v1/images/edits` 已下线。当前网页生图只走 `/api/image/generate`，任务 ID 以 PostgreSQL `job_id` 为主线；未来如果重新提供外部图片 API，应基于同一条 `job_id` 主线实现。

### POST `/v1/chat/completions`

Chat Completions 兼容接口保留路由，但图片生成兼容能力已下线。携带图片生成请求时返回 `410 image_compat_removed`。

请求示例：

```json
{
  "model": "gpt-image-2",
  "messages": [
    {
      "role": "user",
      "content": "生成一张雨伞的图片"
    }
  ],
  "stream": false,
  "n": 1,
  "size": "1024x1024",
  "quality": "high",
  "response_format": "url"
}
```

### POST `/v1/responses`

Responses 兼容接口保留路由，但 `image_generation` 图片生成能力已下线。携带该工具时返回 `410 image_compat_removed`。

请求示例：

```json
{
  "model": "gpt-image-2",
  "input": "生成一张雨伞的图片",
  "tools": [
    {
      "type": "image_generation"
    }
  ],
  "stream": false,
  "n": 1,
  "size": "1024x1024",
  "quality": "high",
  "response_format": "url"
}
```

### GET `/v1/models`

返回兼容模型列表。

## 旧图片会话接口

这些接口是历史图片工作台的服务端会话存储接口，只有当 `storage.image_conversation_storage=server` 时可用。当前业务工作台优先使用 `/api/business/image/conversations`。

### GET `/api/image/conversations`

获取旧图片会话列表。

响应：

```json
{
  "items": []
}
```

### GET `/api/image/conversations/{id}`

获取旧图片会话详情。

响应：

```json
{
  "item": {}
}
```

### PUT `/api/image/conversations/{id}`

保存旧图片会话。

请求体是旧 `imagehistory.Conversation` 对象。

响应：

```json
{
  "item": {}
}
```

### DELETE `/api/image/conversations/{id}`

删除旧图片会话。

响应：

```json
{
  "ok": true
}
```

### DELETE `/api/image/conversations`

清空旧图片会话。

响应：

```json
{
  "ok": true
}
```

### POST `/api/image/conversations/import`

导入旧图片会话。

请求：

```json
{
  "items": [],
  "storage": {
    "backend": "sqlite",
    "imageDir": "data/tmp/image",
    "sqlitePath": "data/image-studio.db",
    "redisAddr": "",
    "redisPassword": "",
    "redisDb": 0,
    "redisPrefix": "",
    "imageConversationStorage": "server",
    "imageDataStorage": "server"
  }
}
```

响应：

```json
{
  "imported": 3
}
```

## 当前挂载路由总表

| 方法 | 路径 | 认证 | 说明 |
| --- | --- | --- | --- |
| POST | `/auth/login` | 公开 | 登录 |
| POST | `/auth/logout` | 可选登录态 | 退出 |
| GET | `/version` | 公开 | 版本 |
| GET | `/health` | 公开 | 健康检查 |
| GET | `/api/config` | 管理员 | 获取底层配置 |
| GET | `/api/config/defaults` | 管理员 | 获取默认底层配置 |
| PUT | `/api/config` | 管理员 | 更新底层配置 |
| POST | `/api/proxy/test` | 管理员 | 测试代理 |
| POST | `/api/integration/test` | 管理员 | 测试集成配置 |
| POST | `/api/integration/newapi/token` | 管理员 | 发现 NewAPI token |
| POST | `/api/integration/sub2api/groups` | 管理员 | 获取 Sub2API 分组 |
| GET | `/api/startup/check` | 管理员 | 启动检查 |
| GET | `/api/runtime/status` | 管理员 | 运行监控 |
| GET | `/api/diagnostics/export` | 管理员 | 导出诊断 |
| POST | `/api/tools/admission-stress` | 管理员 | admission 压测 |
| GET | `/api/sync/status` | 管理员 | 同步状态 |
| POST | `/api/sync/run` | 管理员 | 触发同步 |
| GET | `/api/business/admin/dashboard` | 管理员 | 管理员仪表盘 |
| GET | `/api/business/system-settings` | 管理员 | 获取系统设置 |
| PUT | `/api/business/system-settings` | 管理员 | 更新系统设置 |
| GET | `/api/business/users` | 管理员 | 用户列表 |
| POST | `/api/business/users` | 管理员 | 创建用户 |
| GET | `/api/business/users/{id}` | 管理员 | 用户详情 |
| PATCH | `/api/business/users/{id}/status` | 管理员 | 启用或禁用用户 |
| PATCH | `/api/business/users/{id}/password` | 管理员 | 重置用户密码 |
| PUT | `/api/business/users/{id}/credit` | 管理员 | 设置用户点数 |
| GET | `/api/business/admin/usage` | 管理员 | 所有用户使用记录 |
| GET | `/api/business/admin/jobs` | 管理员 | 所有用户 job 列表 |
| GET | `/api/business/storage/report` | 管理员 | 存储检查 |
| POST | `/api/business/storage/backfill-assets` | 管理员 | 旧资产补录 |
| GET | `/api/business/tracker/summary` | 管理员 | tracker 汇总 |
| GET | `/api/business/api-providers` | 管理员 | API 接入列表 |
| POST | `/api/business/api-providers` | 管理员 | 新增 API 接入 |
| PUT | `/api/business/api-providers/{id}` | 管理员 | 编辑 API 接入 |
| POST | `/api/business/api-providers/{id}/default` | 管理员 | 设为默认 API 接入 |
| POST | `/api/business/api-providers/{id}/test` | 管理员 | 测试 API 接入 |
| DELETE | `/api/business/api-providers/{id}` | 管理员 | 删除 API 接入 |
| GET | `/api/business/me` | UI 登录态 | 当前用户信息 |
| PATCH | `/api/business/me/password` | UI 登录态 | 修改自己密码 |
| GET | `/api/business/credit` | UI 登录态 | 当前用户点数 |
| GET | `/api/business/usage` | UI 登录态 | 当前用户使用记录 |
| GET | `/api/business/jobs` | UI 登录态 | 当前用户 job 列表 |
| GET | `/api/business/jobs/{id}` | UI 登录态 | 当前用户 job 详情 |
| POST | `/api/business/jobs/{id}/cancel` | UI 登录态 | 取消当前用户 job |
| GET | `/api/image/conversations` | UI 登录态 | 旧图片会话列表 |
| DELETE | `/api/image/conversations` | UI 登录态 | 清空旧图片会话 |
| POST | `/api/image/conversations/import` | UI 登录态 | 导入旧图片会话 |
| GET | `/api/image/conversations/{id}` | UI 登录态 | 旧图片会话详情 |
| PUT | `/api/image/conversations/{id}` | UI 登录态 | 保存旧图片会话 |
| DELETE | `/api/image/conversations/{id}` | UI 登录态 | 删除旧图片会话 |
| GET | `/api/business/image/conversations` | UI 登录态 | 业务图片会话列表 |
| DELETE | `/api/business/image/conversations` | UI 登录态 | 清空当前用户业务会话 |
| GET | `/api/business/image/conversations/{id}` | UI 登录态 | 业务图片会话详情 |
| PATCH | `/api/business/image/conversations/{id}` | UI 登录态 | 重命名当前用户业务会话 |
| DELETE | `/api/business/image/conversations/{id}` | UI 登录态 | 删除当前用户业务会话 |
| POST | `/api/image/generate` | UI 登录态 | 业务生图 |
| POST | `/v1/chat/completions` | 兼容 API Key | Chat Completions 兼容接口 |
| POST | `/v1/responses` | 兼容 API Key | Responses 兼容接口 |
| GET | `/v1/models` | 兼容 API Key | 模型列表 |
| GET | `/v1/files/image/{filename}` | 业务图需 UI 登录态 | 图片文件访问 |
| GET | `/*` | 公开 | 前端静态资源和 SPA fallback |

## 历史遗留 / 未挂载接口

以下接口相关 handler 或前端方法仍存在于代码中，但当前 `Server.Handler()` 没有挂载对应路由，调用会返回 404。项目已有测试确认 `/api/accounts` 路由已移除。

| 路径 | 状态 | 说明 |
| --- | --- | --- |
| `/api/accounts` | 未挂载 | 旧账号管理接口，已从当前主路由移除 |
| `/api/accounts/import` | 未挂载 | 旧账号文件导入 |
| `/api/accounts/refresh` | 未挂载 | 旧账号刷新 |
| `/api/accounts/refresh-all` | 未挂载 | 旧账号批量刷新 |
| `/api/accounts/refresh-progress` | 未挂载 | 旧账号刷新进度 |
| `/api/accounts/{id}/quota` | 未挂载 | 旧账号额度 |
| `/api/accounts/image-policy` | 未挂载 | 旧图片账号路由策略 |
| `/api/requests` | 未挂载 | 旧请求日志列表，当前运维页已不应依赖 |

后续新增接口时，建议优先挂到清晰的业务命名空间：

```text
/api/business/...
```

兼容外部 OpenAI 风格调用时，再放到：

```text
/v1/...
```
