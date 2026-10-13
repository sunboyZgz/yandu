# Yandu 服务端 SDK

由 `schemas/storage.openapi.json` 使用 OpenAPI Generator 7.17.0 生成并纳入源码，当前没有发布到 npm。项目密钥只放在业务服务器；包入口拒绝在浏览器环境运行。

```bash
npm ci
npm run build
```

业务项目以本地包依赖 `file:../../sdk/typescript` 接入；可运行示例在 `examples/storage-business`。

```ts
import { Configuration, StorageApi } from '@yandu/storage-server';
const api = new StorageApi(new Configuration({
  basePath: process.env.YANDU_GATEWAY,
  accessToken: process.env.YANDU_PROJECT_KEY,
}));
const capacity = await api.getCapacity({ projectId: 'my-app' });
const response = await api.downloadObjectRaw({ projectId: 'my-app', objectId: 'object-id' });
// Stream response.raw.body. Calling .value() buffers a full Blob.
```

先创建 JSON 上传意图，然后使用 tus-js-client 的标准 Tus 端点；`READY` 表示校验入库成功。完整调用及会话代理说明见仓库 `docs/STORAGE.md`。

再生成代码需要锁定的 oapi-codegen 与 OpenAPI Generator JAR，运行 `scripts/generate-storage-api.py`。正常开发和构建不要求生成器。
