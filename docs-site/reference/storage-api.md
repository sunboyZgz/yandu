---
title: 存储 API、鉴权与错误码
description: v0.2 公共存储 API、权限、标准 Tus、条件版本替换、分页与错误处理。
---

# 存储 API、鉴权与错误码

公共前缀为 `/storage/v1/projects/{projectId}`，本机开发源站在 18770，云端业务网关默认在 19091。以下路径均相对此前缀。

云端运维清单另用 `GET /storage/v1/monitor` 和专用设备监控凭证，供 `yandu list` / `yandu disks` 使用，不属于项目业务 SDK 契约。项目密钥不能枚举全部项目，监控密钥不能操作文件，见[云端监控](../guide/cloud-monitoring.md)。

[下载 OpenAPI 3.0.3 契约](/api/storage.openapi.json)。源码为 `schemas/storage.openapi.json`，Go 与 TypeScript 生成代码已纳入源码。SDK 使用说明见[业务接入](/guide/business-integration)。

## 鉴权与隔离

每次请求携带 `Authorization: Bearer <项目凭证>`。凭证绑定一个项目及操作权限；换 URL 的项目 ID 不会扩展授权。续传逐次鉴权，撤销或停用后活动请求与任务提交也会停止。

本机管理页通过 `/api/v1/storage/projects/{projectId}` 访问同一存储服务，使用本机会话和 CSRF 保护。凭证创建/撤销只在本机管理入口，公共 API 与网关不开放 `/credentials`。业务 SDK 不使用管理地址。

## 接口与权限

| 方法与路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /capabilities`、`GET /capacity` | 能力、配额、预留与卷剩余空间 | `read` |
| `GET /entries`、`GET /entries/{entryId}` | 目录分页、条目 | `read` |
| `POST /folders`、`PATCH /entries/{entryId}` | 建目录、改名、移动 | `metadata` |
| `DELETE /entries/{entryId}` | 软删除，非空目录需 `recursive=true` | `delete` |
| `GET /trash`、`POST /trash/{entryId}/restore` | 回收站及恢复 | `restore` |
| `POST /uploads`、`GET /uploads/{uploadId}` | 创建意图、查询入库状态 | `upload` |
| `POST /tus/`、`HEAD/PATCH /tus/{uploadId}` | 标准 Tus | `upload` |
| `GET/HEAD /objects/{objectId}/content` | 当前原件或指定历史版本 | `read` |
| `GET /objects/{objectId}/versions` | 历史版本 | `read` |
| `GET /objects/{objectId}/derivatives`、`GET/HEAD /derivatives/{derivativeId}` | 当前版本派生物 | `read` |
| `POST /jobs` | 创建媒体任务 | `process` |
| `GET /jobs/{jobId}` | 精确查询源版本、约束和媒体结果 | `read` |
| `POST /jobs/{jobId}/cancel` | 取消排队或运行任务 | `process` |
| `GET /jobs`、`GET /operations` | 任务、删除/恢复操作状态 | `read` |

带媒体预设的上传同时需要 `upload` 和 `process`。接口参数使用虚拟目录、对象、版本 ID；不能传入本机物理路径。

## 目录、分页与修改条件

根目录用 `parentId: ""`。目录列表支持 `parentId`、`cursor` 和 `limit`，有效 limit 为 1–200，默认 100；返回 `items` 与 `nextCursor`，将游标用于下一页。回收站也支持分页。任务与操作列表返回最近最多 100 条，不使用目录游标；单任务查询不受列表条数限制。

新建目录：

```json
{ "parentId": "", "name": "附件" }
```

改名或移动：

```json
{
  "parentId": "目标目录ID或空字符串",
  "name": "新名称",
  "expectedGeneration": 1
}
```

`expectedGeneration` 取最新条目返回的 `generation`。目录移动禁止成环，同目录同名冲突。后台删除/恢复尚未完成时，通过 `/operations` 查询，不把成功响应等同于物理移动已结束。

## 上传意图与 Tus

1. 创建 JSON 意图，携带 `Idempotency-Key`（1–128 字符）。一个逻辑上传在重试时保留相同键及请求参数。
2. 使用响应 `id` 创建 Tus：`Upload-Length` 是同一个实际长度，`Upload-Metadata: intentId <id的Base64>`，`Tus-Resumable: 1.0.0`。
3. 依据 Tus `Location` 续传，HEAD 读取偏移，PATCH 使用 `application/offset+octet-stream` 和 `Upload-Offset`。
4. 查询 `/uploads/{uploadId}`，只有 `READY` 才表示校验、正式入库和数据库提交成功。

```json
{
  "parentId": "",
  "name": "原件.bin",
  "length": 1048576,
  "processing": { "presets": [] }
}
```

创建返回 `id`、`objectId`、`versionId`、`tusEndpoint`。相同幂等键重试返回已有意图 ID；同键不同参数返回冲突。响应丢失后先查询已有意图状态，已 `READY` 时直接使用其对象与版本，不重新创建 Tus。

Tus 字节完成后还可能处于 `pending` 或 `committing`；业务继续查询。`failed` 或 `cancelled` 需要检查原因，不能通过继续 PATCH 把关闭的意图改成就绪。标准 Tus 的下载、拼接和 termination 扩展当前未开启。

请求需要预先给出长度，不支持 deferred length。服务核对暂存和正式内容的长度、SHA256；当前意图没有调用方上传预期 SHA256 的参数。

## 条件替换与下载

替换意图在上面增加同一 `objectId` 与当前 `expectedVersionId`。新版本提交时再次比较条件；并发替换只有一个获胜，新版本就绪前保留旧版本服务。

原件下载可加 `?versionId=<历史版本ID>`。支持 HEAD、Range、ETag、If-Match、If-None-Match 和 If-Range；业务中转保留 200/206/304/412/416 等状态及相关头。

被删除的对象不提供历史原件或派生物读取；派生物限定未删除对象的当前版本。HLS 清单将分片地址改为受鉴权派生物路径。

## 能力、任务与容量

先读 `/capabilities` 判断 `mediaReady`、`presets` 和新增的 `outputConstraintCapabilities`，不要依据扩展名或平台名称推测组件可用。处理任务格式、产物和 HLS 接入见[媒体指南](/guide/media)。

项目上限由本机管理配置 `mediaPolicy.mp4MaxFileBytes`，业务任务不能覆盖。`outputConstraintCapabilities.presets["mp4-720-v1"]` 声明 `constraintScope: "project"`、`configuredMaxFileBytes`（未配置为 null）、`maxAllowedFileBytes`、策略和规则版本。其他预设声明不支持新约束。

带约束的上传在 `processing.outputConstraints` 中提供策略；原件 `READY` 后 `/uploads/{uploadId}.jobs` 返回任务 ID。后续 `POST /jobs` 请求增加 `expectedVersionId` 和根级 `outputConstraints`，带约束时必须提供 `Idempotency-Key`。返回 `{ok: true, jobs: [...]}`，按 `GET /jobs/{jobId}` 查询 `queued/running/succeeded/failed/cancelled`；具体结构以生成契约为准。

容量的 `usedBytes` 包括原件各版本、回收站及派生物，`reservedBytes` 包括正在进行的工作。创建上传与媒体任务还检查共享底层卷空间，所以可用空间不能只按 `quotaBytes - usedBytes` 估算。

## 错误与重试

常见响应格式：

```json
{ "error": { "code": "NAME_CONFLICT", "message": "name already exists" } }
```

| HTTP / 错误码 | 处理方式 |
| --- | --- |
| 401 `UNAUTHORIZED` / `CREDENTIAL_REVOKED` | 核对凭证、归属与撤销状态；不自动扩权 |
| 403 `SCOPE_DENIED` / `PROJECT_DISABLED` | 检查操作权限或项目是否启用/隔离 |
| 404 `NOT_FOUND` / `PROJECT_NOT_FOUND` | ID、项目、删除或就绪状态不符 |
| 409 `NAME_CONFLICT` / `VERSION_CONFLICT` | 刷新元数据或当前版本，解决冲突 |
| 409 `FOLDER_CYCLE` / `DIRECTORY_NOT_EMPTY` | 修改目标，或明确递归范围 |
| 409 `IDEMPOTENCY_CONFLICT` / `UPLOAD_CLOSED` | 保留正确逻辑上传键，先查询意图 |
| 409 `OPERATION_PENDING` | 查询操作，等待移动结束 |
| 400 `PROCESS_PRECONDITION_REQUIRED` / `INVALID_REQUEST` | 补齐版本与幂等键；不能传任务级大小上限 |
| 422 `UNSUPPORTED_OUTPUT_CONSTRAINT` / `INVALID_OVERFLOW_POLICY` | 检查预设和策略，不能忽略后回退 |
| 422 `PROJECT_MEDIA_POLICY_REQUIRED` | 在本机配置并应用项目上限 |
| 任务 `OUTPUT_LIMIT_EXCEEDED` | 读取实测大小与上限；原件不变，不把查询 HTTP 200 当作处理成功 |
| 任务 `NO_VALID_TRIM` / `INVALID_MEDIA_RESULT` | 没有合格片段或产物不完整，不发布候选 |
| 任务 `VERSION_CHANGED` / `JOB_CANCELLED` | 旧源失效或已取消，不自动恢复提交 |
| 任务 `PROCESSING_TIMEOUT` / `WORKSPACE_LIMIT` | 时间或工作区预算不足，不能冒充大小合格 |
| 503 `MEDIA_UNAVAILABLE` | 补齐媒体组件或去掉处理预设 |
| 503 `DEVICE_OFFLINE` | 网关无法连接设备，结合 `Retry-After: 5` 重试 |
| 503 `VOLUME_CHANGED` / `INTEGRITY_QUARANTINE` | 由本机管理员检查卷及维护核验 |
| 507 `INSUFFICIENT_STORAGE` | 检查配额、预留和底层可用空间 |

意图或异步任务的失败不一定表现为当前查询 HTTP 错误，仍需检查响应中的 `state` 和 `error`。公共错误不会返回内部物理路径；详细排查通过本机管理页和日志。
