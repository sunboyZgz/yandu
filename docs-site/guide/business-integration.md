---
title: 业务服务与 TypeScript SDK
description: 用服务端 SDK、业务会话和流式代理接入 Yandu，运行独立业务示例并导入已有文件。
---

# 业务服务与 TypeScript SDK

业务通过项目 ID 和凭证访问存储。项目密钥只留在业务服务器，浏览器先通过业务自己的登录会话，再请求业务的上传/下载中转。

完整云端业务需要先[启动云端](./cloud-startup.md)、[启动本机并配对](./getting-started.md)，再接入本页的业务服务。

## 选择网关地址

| 业务运行位置 | `YANDU_GATEWAY` |
| --- | --- |
| 与开发实例在同一台机器 | `http://127.0.0.1:18770` |
| Ubuntu 云端与默认网关同机 | `http://127.0.0.1:19091` |
| 显式配置的受信内网 | 带可信 TLS 证书、允许网段的真实网关地址 |

SDK `basePath` 填网关根地址，不附加 `/storage/v1`；生成的接口自己拼接项目路径。Docker 及受信内网配置见[云端接入指南](./cloud-startup.md#docker-业务接入)。

## 业务示例的端口

示例固定监听 `127.0.0.1:3000`，当前没有 `PORT` 环境变量。要改成其他端口，修改 `examples/storage-business/server.mjs` 的 `app.listen` 和两处 Origin 校验，再重启。`YANDU_GATEWAY` 调整上游存储地址，不调整业务监听。

云端网关改端口时同步更新本页的 `YANDU_GATEWAY` / SDK `basePath`，配置步骤见[云端网关端口](./ports.md#修改业务网关端口)。

## 构建本地 SDK

SDK 位于 `sdk/typescript`，来自已提交的 OpenAPI 生成代码，**当前未发布到 npm**。在仓库根目录：

```bash
npm --prefix sdk/typescript ci
npm --prefix sdk/typescript run build
npm --prefix examples/storage-business ci
```

普通开发不需要 Java 或运行生成器。独立示例通过 `file:../../sdk/typescript` 使用本地包。其他业务可调整相对路径使用 `file:` 依赖，或先在 SDK 目录运行 `npm pack`，再安装生成的包。

## 运行独立业务示例

先在管理页创建并启用 `demo`，申请 `read` 和 `upload` 凭证。示例用于本机验证。

macOS，在仓库根目录：

```bash
export YANDU_GATEWAY='http://127.0.0.1:18770'
export YANDU_PROJECT_ID='demo'
export YANDU_PROJECT_KEY='替换为服务器端项目密钥'
export BUSINESS_PASSWORD='替换为业务示例登录密码'
export SESSION_SECRET="$(node -e 'process.stdout.write(require("node:crypto").randomBytes(32).toString("hex"))')"
npm --prefix examples/storage-business start
```

Windows PowerShell：

```powershell
$env:YANDU_GATEWAY='http://127.0.0.1:18770'
$env:YANDU_PROJECT_ID='demo'
$env:YANDU_PROJECT_KEY='替换为服务器端项目密钥'
$env:BUSINESS_PASSWORD='替换为业务示例登录密码'
$env:SESSION_SECRET=(node -e 'process.stdout.write(require("node:crypto").randomBytes(32).toString("hex"))')
npm --prefix examples/storage-business start
```

环境值由服务端管理；示例占位密钥和密码必须替换。打开业务页输入业务密码后，上传一个小原件、刷新文件、下载并检查容量。关闭示例终端可停止服务。

示例用生成 SDK 查询容量、http-proxy 中转 Tus/下载、tus-js-client 传输。只转发固定项目范围，禁止浏览器访问凭证管理端点。它是单账号、内存会话示例；生产接入应替换为已有业务账号、持久会话及 HTTPS。

## 在上传时自动压缩 MP4

先在 Yandu 管理页为项目配置 MP4 产物上限，示例凭证同时需要 `upload`、`read`、`process`。业务页可勾选 MP4 并明确选择是否允许裁掉尾部；页面使用项目固定上限，不提供任务级大小输入。

示例分别显示原件入库和压缩状态，原件 `READY` 后按上传状态中的任务 ID 查询，成功时提供派生物下载及结果摘要。控制查询通过业务服务端的生成 SDK，文件继续流式中转；浏览器不接触项目密钥。

```ts
const upload = await api.getUploadIntent({ projectId, uploadId });
for (const ref of upload.jobs ?? []) {
  const job = await api.getJob({ projectId, jobId: ref.id });
  if (job.state === 'succeeded' && job.result) {
    // 通过 downloadDerivativeRaw() 中转结果，再独立核验大小、摘要及播放。
    console.log(job.result.outputBytes, job.result.outputDurationSeconds);
  }
}
```

能力、策略与完整结果字段见[媒体指南](./media.md)。批量导入工具仍默认只保存原件，不自动裁剪来源。

## 调用 SDK 控制接口

```ts
import { Configuration, StorageApi } from '@yandu/storage-server';

const projectId = process.env.YANDU_PROJECT_ID!;
const api = new StorageApi(new Configuration({
  basePath: process.env.YANDU_GATEWAY,
  accessToken: process.env.YANDU_PROJECT_KEY,
}));

const capacity = await api.getCapacity({ projectId });
const folder = await api.createFolder({
  projectId,
  folderInput: { parentId: '', name: '附件' },
});
const entries = await api.listEntries({ projectId, parentId: folder.id });
```

容量/列表需要 `read`，新建目录需要 `metadata`。SDK 包入口拒绝在浏览器环境运行；管理页和业务网页都不应导入这个带项目密钥的服务端实例。

## 流式下载与标准 Tus

大文件下载用 `downloadObjectRaw()` 返回的原始响应流：

```ts
const response = await api.downloadObjectRaw({ projectId, objectId });
const status = response.raw.status;
const headers = response.raw.headers;
const body = response.raw.body;
// 将 status、必要响应头和 body 流转发给业务客户端。
```

不要调用 `.value()`、`.blob()`、`.arrayBuffer()` 把整个文件载入内存。中转 HEAD、Range、ETag 和条件请求时保留相应状态与头部。

上传先调用 `createUploadIntent({ projectId, idempotencyKey, intent })`，再用 tus-js-client 操作标准 Tus 端点；续传 metadata 为 `intentId`。浏览器请求经业务会话中转，服务端附加 Bearer 凭证，随后查询上传状态直到 `READY`。完整状态流程及并发版本替换见[存储 API](/reference/storage-api)。

业务中转需要覆盖 Tus 创建、HEAD、PATCH，以及 HLS 清单和分片。不要用有较小 JSON/body 限制的中间件提前读取整个上传。独立示例是可运行的流式接入参考。

## 导入已有文件

先创建空存储项目。业务示例的 `import.mjs` 遍历指定来源，通过 SDK 和标准 Tus 导入目录与文件，保留来源，不扫描或接管整盘。

设置 `YANDU_GATEWAY`、`YANDU_PROJECT_ID`、`YANDU_PROJECT_KEY`，凭证需要 `read`、`upload`、`metadata`。macOS 示例：

```bash
cd examples/storage-business
node import.mjs '/absolute/path/to/source'
```

Windows 示例：

```powershell
cd examples\storage-business
node .\import.mjs 'D:\待导入资料'
```

拒绝链接及多硬链接文件。导入创建新对象 ID，旧只读项目不会自动转换。同名重跑会冲突；当前不是跨次运行的同步器或自动去重工具，先核对已导入项再决定重试。
