---
title: 图片、HEIC、视频与任务
description: v0.2 媒体预设、可选处理参数、HEIC 解码、派生产物及 HLS 鉴权接入。
---

# 图片、HEIC、视频与任务

媒体处理由 API 参数控制。默认 `processing.presets` 为空，只保存原件；也可以在原件就绪后创建任务。开发模式使用 `--no-media` 时，`mediaReady: false`，处理请求返回 `MEDIA_UNAVAILABLE`。

## 五个处理预设

| 预设 ID | 适用输入 | 产物 |
| --- | --- | --- |
| `thumbnail-256-v1` | 图片，包括已支持的 HEIC | 最长边不超过 256 的 `thumbnail.webp` |
| `webp-1280-v1` | 图片，包括已支持的 HEIC | 最长边不超过 1280 的 `image.webp` |
| `video-cover-v1` | 受支持视频 | 首帧 `cover.jpg`，不超过 1280×720 |
| `mp4-720-v1` | 受支持视频 | H.264、可选 AAC 音轨的 `video.mp4`，不超过 1280×720 |
| `hls-720-v1` | 受支持视频 | HLS VOD `index.m3u8` 和 TS 分片，不超过 1280×720 |

保持比例，不放大原件。每个任务另外生成归一化的 `metadata.json`，提供尺寸、图片方向归一化/输出色彩空间，或视频时长及编码信息。原件与元数据作为不同受鉴权对象读取。

处理器按真实文件签名识别输入，改扩展名不能改变格式。图片路径接受受支持的 JPEG、PNG、WebP、GIF、TIFF 和 HEIC；视频路径覆盖 MP4/MOV、Matroska、AVI、MPEG/TS 中可解码的输入。SVG 不进入图片处理链；损坏文件、异常尺寸或不受支持的编码可能使任务失败。

## 项目级 MP4 大小上限

在项目「项目压缩设置」启用上限，填写字节数或十进制 MB，点击「保存并应用」。例如 10 MB 表示 **10,000,000 字节**；输入必须为安全正整数且不超过项目配额。停用项目可先保存，启用时应用。

每个项目分别配置 `mediaPolicy.mp4MaxFileBytes`。同一项目的新 MP4 任务使用同一上限，任务不能传 `maxFileBytes` 覆盖。上限只计算 `video.mp4` 的完整容器、索引、音视频等字节，不计算原件、封面或 `metadata.json`；工作区预算与项目配额另行检查。

未配置上限的旧项目保留完整压缩行为。项目规则在提交时冻结，修改仅影响后续提交；上传续传和同键重试继续使用原快照，历史产物不会自动重压。

| 超限策略 | 行为 |
| --- | --- |
| 未填写或 `reject` | 完整压缩超限则任务失败，原件保留 |
| `keep-start` | 明确允许保留开头、裁掉尾部；完整结果符合上限时仍不裁剪 |

上传处只展示项目上限并选择策略。后台不会再次等待人工确认，也不会在未授权时自动裁剪。当前仅 `mp4-720-v1` 支持新约束；对图片、封面或 HLS 填约束会在提交阶段被拒绝。

## 在管理页选择

进入项目「文件」，在媒体预设里勾选与输入相匹配的选项，然后上传。对已经存在的文件，也可先勾选预设，再点该行「处理所选预设」。

在「媒体任务」查看状态。任务成功后打开文件行的「派生文件」查看及下载；任务「详情」显示源版本、规则快照、实际字节数、SHA256、原始与保留时长，并支持取消；业务错误不暴露物理路径。

## 上传时处理

创建上传意图：

```json
{
  "parentId": "",
  "name": "照片.heic",
  "length": 102400,
  "processing": {
    "presets": ["thumbnail-256-v1", "webp-1280-v1"]
  }
}
```

`length` 使用实际字节数。请求需要 `upload`，指定预设时同时需要 `process`；按[存储 API](/reference/storage-api)完成标准 Tus 上传。原件 `READY` 后处理任务开始排队，不必等待所有产物才读取原件。

每次意图最多五个预设，不重复填写。

有项目上限的视频上传示例：

```json
{
  "name": "video.mp4",
  "length": 25000000,
  "processing": {
    "presets": ["mp4-720-v1"],
    "outputConstraints": {
      "mp4-720-v1": { "onOverflow": "keep-start" }
    }
  }
}
```

先读取 `/capabilities.outputConstraintCapabilities`，确认 `constraintScope: "project"`、`configuredMaxFileBytes` 和支持策略。项目未配置上限、组件未就绪或约束不支持时，提交明确失败，不回退成无约束成功。

原件 `READY` 时，上传状态响应的 `jobs` 返回稳定的 `{id, preset, sourceVersionId}`。查询 `GET /jobs/{id}`，等待 `succeeded` 后使用 `result.derivativeId` 下载产物；`failed`、`cancelled` 是媒体终态，原件仍可保留。上传意图查询需要 `upload`，媒体任务查询需要 `read`。


## 原件就绪后提交

```http
POST /storage/v1/projects/demo/jobs
Authorization: Bearer <服务端项目凭证>
Content-Type: application/json
```

```json
{
  "objectId": "实际对象ID",
  "presets": ["video-cover-v1", "mp4-720-v1", "hls-720-v1"]
}
```

提交需要 `process`，查询需要 `read`。带 `outputConstraints` 的后续任务须同时提供当前 `expectedVersionId` 和 `Idempotency-Key`（1–128 字符），策略写在请求根级 `outputConstraints`，大小仍由项目提供。

成功响应保留 `ok: true`，新增 `jobs` 任务引用。相同键与参数返回同一任务；同键不同源版本、预设或策略返回 409。响应丢失后用同键重试取得任务 ID，再查询 `GET /jobs/{id}`；不依靠最近 100 条列表推测。取消用 `POST /jobs/{id}/cancel`，需要 `process`。新逻辑请求使用新键；未使用新参数的旧调用仍支持。

Worker 默认单任务并发，采用持久状态和租约，暂时故障最多自动尝试三次，确定性超限或无有效片段立即失败。等待处理不会阻塞后续上传入库。任务超时、线程、Node 堆、输出及空间预算有约束；**OS 硬资源隔离尚未实现**。

## 压缩结果与独立核验

处理顺序为完整压缩、核对真实大小、必要时沿时间线裁剪、重新生成完整 MP4、探测与完整解码，再经 SFTPGo 入库。裁剪采用有界搜索，不保证恰好填满上限或保留最长片段；不会按字节截断文件，也不会仅靠码率估算宣称合格。

成功任务返回 `sourceVersionId`、`sourceSha256`、`processingVersion`、`policyRevision`、`appliedConstraints`，以及以下 `result` 字段：

| 字段 | 含义 |
| --- | --- |
| `derivativeId` | 受项目鉴权的 MP4 派生物 ID |
| `originalDurationSeconds`、`outputDurationSeconds` | 原始及最终产物实测时长 |
| `fullCompressedBytes`、`outputBytes` | 完整压缩及最终产物的实测字节数 |
| `outputSha256`、`mime` | 最终产物摘要与类型 |
| `trimmed`、`retainedRange` | 是否裁掉尾部；保留范围起点为 0，终点取最终实测时长 |

业务应下载产物并核对大小、SHA256、时长和解码，再冻结产物。旧任务缺少的摘要不伪造，详情可能没有 `result`；原生测试与未执行项见[MP4 验收记录](/reference/media-acceptance)。

## HEIC 主图、方向和颜色

HEIC 使用锁定 libheif/libde265 的原生 `heif-thumbnailer` 选择主图并应用方向变换，再由 Sharp 生成 WebP 和 sRGB 输出。macOS 与 Windows 采用相同处理流程，避免依赖不同 Sharp 预编译包是否带 HEIC 支持。

当前已在 macOS 验证 HEIC 多图主图、方向 6、sRGB 颜色与不放大；Windows 必须在原生组件构建后运行同一套夹具。不能以交叉编译或普通 PNG 转码代替 HEIC 验收，见[原生验证记录](/reference/storage-verification)。

## HLS 的业务访问

`GET /objects/{objectId}/derivatives` 返回清单与分片的派生物 ID。读取清单时，Yandu 将分片引用改为同项目受鉴权的派生物 URL。

业务服务需要同时转发清单和分片路径，使用自己的浏览器会话鉴权，并在服务端附加项目凭证。不要将凭证写入 m3u8 或分片 URL。HLS 播放器读取分片同样经过业务中转；只代理清单不能完成播放。

## 删除、替换和故障

任务提交和产物入库时重查对象当前版本、删除状态、项目和凭证。删除后原件及派生物立即不可读，旧任务不能恢复文件。新版本有自己的处理任务；旧版本的派生物不会作为当前版本提供。

任务失败时先核对组件版本、输入格式、配额和空间，再看管理页任务错误。未提交的产物有持久清理记录；后端恢复后清理前会保留预留空间。备份、隔离恢复及迁移按[维护指南](./storage-maintenance.md)执行。
