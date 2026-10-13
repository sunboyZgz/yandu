# 独立业务服务示例

Node.js 22；先在本机管理页创建并启用一个存储项目，再创建项目凭证。云端启动 storage-only 网关或在本机测试时将 `YANDU_GATEWAY` 设为 `http://127.0.0.1:18770`。

```sh
npm ci
YANDU_GATEWAY=http://127.0.0.1:19091 \
YANDU_PROJECT_ID=my-app YANDU_PROJECT_KEY=服务器端项目密钥 \
BUSINESS_PASSWORD=业务登录密码 SESSION_SECRET=至少32字符的随机会话密钥 npm start
```

PowerShell 用 `$env:变量名="值"` 设置相同环境变量，再运行 `npm start`。访问 http://127.0.0.1:3000。

示例使用 OpenAPI Generator 的 TypeScript SDK 查询容量、http-proxy 流式中转内容和 Tus、tus-js-client 上传。浏览器只收到项目 ID 和业务会话 Cookie。生产接入请使用已有业务账号、持久会话存储和 HTTPS；当前单账号、内存会话仅用于独立接入验证。

大文件下载使用 `await api.downloadObjectRaw({...})` 返回值的 `raw.body` 进行流式转发。不要调用生成的 `.value()` 或 `.blob()` 把整个对象加载进服务端内存。恢复、目录操作和媒体处理均调用 SDK 控制接口。
