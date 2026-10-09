# 已实现接口与 CLI

所有管理接口仅监听 `127.0.0.1:18765`，只接受这个精确 Host。浏览器必须具有本机会话，非 GET 操作必须带同源 Origin 和 `X-CSRF-Token`；CLI 使用 ACL 限定的 `local.token` 文件访问。正文最多 1 MiB，拒绝未知 JSON 字段。没有 CORS 放行。

| 方法 / 路径 | 行为 |
|---|---|
| GET /api/v1/health | 最小进程健康信息，无项目/凭据 |
| POST /api/v1/launch | 本机 bearer 取得单次 60 秒票据 |
| POST /api/v1/session | 票据换 HttpOnly SameSite Strict 会话与 CSRF |
| GET /api/v1/session | 已登录浏览器读取当前 CSRF |
| GET /api/v1/status | 脱敏项目、组件进程、授权站点、操作、版本 |
| GET /api/v1/projects | 脱敏项目列表 |
| POST /api/v1/projects | 保存新草稿 `{project,password?,expectedRevision:0}` |
| PUT /api/v1/projects/{id} | 修改草稿/目标配置，检查项目 expectedRevision |
| POST /api/v1/projects/{id}/validate | 路径、ACL 可读性及资源授权校验 |
| POST /api/v1/projects/{id}/apply | `{expectedRevision,confirmPublication:true}`；202 operationId |
| POST /api/v1/projects/{id}/disable | 本地先阻断，再撤销隧道与同步拒绝入口 |
| POST /api/v1/projects/{id}/remove | 停用后删除映射，保留文件和旧路径拒绝记录 |
| POST /api/v1/projects/{id}/verify | `{password}` 验证已接入的密码资源 |
| POST /api/v1/roots | `{path}` 明确授权独立素材目录 |
| GET /api/v1/files?projectId=blog&path=xxx&offset=0 | 项目目录分页 100 项，不提供任意文件正文读取 |
| POST /api/v1/directories | `{projectId,path}` 在项目内创建一个子目录 |
| POST /api/v1/connection/import | `{bundle,passphrase}` 解密并验证连接包，项目保持停用 |
| POST /api/v1/connection/test | 验证主机指纹及云端站点授权，读取回执版本 |
| POST /api/v1/routes/release | `{siteId,routeId,confirm:true}`；只释放已停用旧路径 |
| GET /api/v1/config/export | 导出不含密码/连接材料的草稿配置 |
| POST /api/v1/config/import | 导入草稿；密码项目需重新设置独立密码 |
| GET /api/v1/operations/{id} | 异步操作阶段及真实错误 |
| GET /api/v1/diagnostics | 脱敏诊断，无目录、设备私钥、token、会话 |

项目例：

```json
{"project":{"id":"blog","displayName":"个人博客","siteId":"main","projectBase":"/blog","rootPath":"D:/oss/blog","resourcePrefixes":["/blog/xxx/"],"publication":"disabled","accessPolicy":"public_read","cachePolicy":"revalidate","contentPolicy":"media_only"},"expectedRevision":0}
```

`passwordHash`、`generation`、`probeToken` 不返回给 UI，也不接受来自客户端的派生状态作为实际生效依据。已有项目修改会保留内部密码散列。站点与基础路径变更需要先停用、删除并释放旧范围；目录迁移和同站点资源前缀变更可直接编辑/应用。

错误结构：`{"error":{"code":"REVISION_CONFLICT","message":"…"}}`。错误码涵盖路径、授权、reparse point、URL 歧义、路由冲突、受保护页面、隧道离线、SSH 指纹、Nginx 校验、入口核验、版本冲突及组件缺失。409 表示版本/路由冲突；403 表示权限/会话；404 表示不存在；其余输入或应用错误返回 400。

## CLI

```bash
yandu [--state-dir /absolute/state] ui
yandu serve
yandu status
yandu doctor
yandu connection import main.yandu-profile
yandu root grant D:/oss/blog
yandu project add --id blog --name 个人博客 --site main \
  --root D:/oss/blog --base /blog --prefix /blog/xxx/
yandu project list
yandu project apply blog --confirm-publish
yandu project disable blog
yandu project remove blog
yandu route release main blog-412212693349
yandu config export projects.json
yandu config import projects.json
```

`apply` 明确要求发布确认。命令返回 operationId，由状态页或 `/operations/{id}` 查看结果。`ui` 检查/启动服务并以一次性 fragment 票据打开浏览器；重复运行不创建第二个 Agent。口令从终端隐藏输入，不放命令参数或环境变量。

## 云端协议

SSH 只执行 root 拥有的固定 helper。principal 来自固定 authorized_keys/sudo 绑定，不读取请求体里的设备声明。请求为 `{"action":"apply","manifest":{…}}`，也支持 `status` 和 `result`。没有目录、上游、端口或 Nginx 原文输入。

清单：schemaVersion=1、operationId（32 个随机十六进制字符）、siteId、expectedCloudRevision、sourceRevision、routes；释放另携带 `release` 路由 ID 数组。最多 100 路由、100 释放记录，正文最多 64 KiB。routeId 由项目 ID 与字面前缀 SHA-256 的前 12 位确定。

回执包含 operationId/siteId/revision/generation/verified/error。云端 verified 表示实际 Nginx 返回相应 generation；客户端必须再以随机资源内容验证整条 HTTPS 数据链路，才显示“已生效”。通用网站 200 不算通过。

SQLite 是本地唯一目标配置来源。generated 配置用于执行，不可手改并期待反向导入。frpc 固定运行文件仅由已校验候选替换，以保证 reload 真正读取新版本。
