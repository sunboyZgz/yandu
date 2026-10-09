# 檐渡 · Yandu：本地资源穿透工具落地实施方案

方案版：2.0｜目标产品版本：v0.1｜日期：2026-10-08

交付性质：开发设计与接口契约，不是已实现的软件。本包不含可运行的 Yandu 客户端、安装器或云端助手；不声称已完成 Windows、Nginx、frp 或公网联调。组件资料来自官方文档，产品行为、接口和验收标准是本方案提出的设计。

建议名称：**檐渡（Yandu）**。用“把屋檐下的本地资源渡到网站中”的意象命名，仓库和命令暂用 `yandu`，不表示商标、域名或所有包名已完成可用性核验。

## 1. 已确定的产品边界

Windows 是目录与项目配置的管理中心；配置页面只在本机浏览器运行。云端承载业务网站、公共资源入口和隧道服务，不建设 Yandu 公网管理网页。文件保留在 Windows，默认不复制到云端，不提供远程磁盘挂载。

最终访问语义必须是：

```text
GET https://example.com/blog
    → 云端博客页面

页面里的图片：/blog/xxx/xxxxx/xx.png
GET https://example.com/blog/xxx/xxxxx/xx.png
    → 云端资源路径分流 → 内网穿透 → Windows 文件服务
    → D:/oss/blog/xxx/xxxxx/xx.png
```

用户示例中的 HTTP 用来说明路径；正式部署默认 HTTPS，路径结构不改变。`xxx` 是示例资源前缀，不预设为所有项目的真实目录名。

不可变要求：

- Windows 管理页只监听 `127.0.0.1`，不监听 LAN、公网、`0.0.0.0` 或全 IPv6 地址；管理 API、目录浏览、日志、内部配置 API 永不成为穿透目标。
- 页面、业务 API、前端构建资源仍由现有云端网站处理；只转发显式登记的本地资源路径，不整体接管 `/blog`。
- Windows 本地文件服务必须存在：普通图片文件本身不是 HTTP 服务；frp 不替代此职责。
- 一套安装支持多个项目；在已授权网站内新增资源前缀后，工具自动应用本地配置及云端规则，无需手工 SSH 编辑配置。
- 目录移动与普通文件更新不重启云端；新增云端路径可以触发经过校验的 Nginx 配置重载，不宣称所有变更绝对无中断。
- 关闭浏览器不停止后台服务；删除映射不删除文件；升级卸载默认保留业务资源。
- 管理页私密不等于资源私密。公开前明确确认，停用与收紧权限优先阻断，不能失败后回到公开状态。
- 本工具发布的是资源入口，不决定文章发布；blog 继续由既有 CLI 控制已发布素材，不能绕过显式发布规则。

## 2. 本版相对旧方案的修正

| 旧方案假设 | 本版决策 |
| --- | --- |
| 默认每项目独立资源子域名 | 默认同域名、显式资源路径分流 |
| 依赖泛域名 DNS 和通配符证书 | 复用现有站点域名及证书，不新增通配符前置条件 |
| 仅客户端注册即可完成新增项目 | 自定义路径须同时更新云端 Nginx 与 frpc |
| 云端只安装组件，没有配置接收机制 | 增加按需执行的受限云端配置助手，通过 SSH 传输声明式清单 |
| 云端 Nginx/Caddy 同时作为首版选项 | v0.1 自动化先支持宿主机 Nginx；云端 Caddy 适配后续独立交付 |
| 资源 Host 直接对应本地根目录 | Host + 资源前缀筛选；Windows 只去除一次项目基础前缀 |

云端选 Nginx、Windows 选 Caddy，是分工而非重复：前者接入已有网站并分流，后者读取 Windows 磁盘。首版不需要 frp-panel、Pangolin、Docker Desktop、外部数据库或消息队列。

## 3. 首版范围与正式支持矩阵

建议验收基线：Windows 11 x64 + Ubuntu 24.04 LTS x64 + 宿主机原生 Nginx。此处是目标测试环境，不是已经验证的兼容性声明。Windows 其他版本、ARM、容器化 Nginx、宝塔/1Panel 自动集成、云端 Caddy 等在通过各自验收前不标为正式支持。

v0.1 必须包含两端安装/升级/卸载契约、本地 Web UI、目录与项目管理、多资源前缀、声明式云端同步、静态文件读取、状态与诊断、启动器、开机恢复、完整发布包和测试。

首版访问策略：默认停用；明确启用后公开只读；可选基础密码保护，必须通过与原网站 Authorization 行为的联合测试。业务登录、签名链接和上传 API 不冒充基础密码保护的现成功能。

首版不做任意 TCP/UDP 代理、多租户、公网控制台、网页素材编辑器、S3/SMB、双向文件同步、CDN/云端副本、视频转码。保留扩展接口而不先实现多套后端。

## 4. 架构与技术分工

### 4.1 资源链路

```text
浏览器 ── /blog ───────────────────→ 云端网站
   │
   └── /blog/xxx/xxxxx/xx.png
         ↓
      云端 Nginx :443
      只处理已登记资源前缀
         ↓ 保留原 Host 与 URI
      frps 127.0.0.1:18080
         ↓ 经过认证并验证身份的 TLS 隧道
      Windows frpc
         ↓
      Windows Caddy 127.0.0.1:18766
      检查 Host、资源前缀、方法与内容策略
      去掉 /blog，再定位文件
         ↓
      D:/oss/blog/xxx/xxxxx/xx.png
```

frp HTTP 路由支持 `customDomains` 与 `locations`，后者是最长前缀匹配；不会自动把文件夹发布为 HTTP，也不会自动修改 Nginx。[S1]

### 4.2 配置链路

```text
本机浏览器 127.0.0.1:18765
           ↓
       yandu.exe
       ├── SQLite：项目、版本、应用结果
       ├── Caddy 配置生成与应用
       ├── frpc 配置生成与 reload
       └── 受限 SSH 会话，发送 JSON 声明
                    ↓
             云端 OpenSSH
                    ↓ 固定命令，不提供交互 shell
             yandu-cloud 配置助手
             校验站点授权与清单 → 生成受管片段
             → nginx -t → reload → 验证及回执
```

SSH 仅用于配置及状态，不是资源数据通道。SSH 不可达但现有 frp 链路仍在时，已有资源不应因为配置通道失败而停服。云端助手按调用启动，不新增公网 HTTP 管理 API。[S6]

### 4.3 组件

| 自研/复用 | 模块 | 实现 |
| --- | --- | --- |
| 自研 | Windows Agent、CLI、启动器 | Go；Windows Service；同一套领域逻辑 |
| 自研 | 本地管理页 | React + TypeScript，构建后嵌入程序 |
| 自研 | 云端配置助手 | Go；受限 SSH 命令；窄权限配置应用 |
| 自研 | 安装与升级整合 | PowerShell/Bash，预编译包 |
| 复用 | 文件服务 | Windows Caddy |
| 复用 | 穿透 | frpc/frps，不 fork 协议 |
| 复用 | 云端入口 | 宿主机 Nginx |
| 复用 | 证书管理 | 优先既有机制；空服务器可用 Certbot webroot |

前端构建产物通过 Go embed 分发。最终 Windows 使用者不需要 Node、pnpm、Go 编译器、Python 或 Docker Desktop。[S9]

## 5. 路由模型：以实际 URL 为准

### 5.1 项目模型

```json
{
  "id": "blog",
  "displayName": "个人博客",
  "siteId": "main",
  "projectBase": "/blog",
  "rootPath": "D:/oss/blog",
  "resourcePrefixes": ["/blog/xxx/"],
  "publication": "disabled",
  "accessPolicy": "public_read",
  "cachePolicy": "revalidate",
  "contentPolicy": "media_only"
}
```

`siteId` 关联经云端安装时授权的真实域名；Windows 不可通过 JSON 传入一个新域名就取得管理权。

`projectBase` 是本地路径转换时去掉的部分。`resourcePrefixes` 是哪些请求有资格进入文件链路。二者不能混同：

```text
匹配：/blog/xxx/xxxxx/xx.png 属于 /blog/xxx/
去除：只去掉 /blog
余下：/xxx/xxxxx/xx.png
读取：D:/oss/blog/xxx/xxxxx/xx.png
```

云端和 frp 不改写 URI。Windows 只去除一次项目基础前缀。Caddy 的 `uri strip_prefix` 与 `file_server` 可提供这两个基础操作；实际安全校验由产品契约和测试补齐。[S4][S5]

### 5.2 路由规则

| 请求 | 结果 |
| --- | --- |
| `/blog`、`/blog/`、`/blog/posts/123` | 保留云端网站行为 |
| `/blog/api/...` | 保留原业务接口，属于受保护路径 |
| `/blog/xxx/xxxxx/xx.png` | 穿透至 blog 目录 |
| `/blog/xxxevil/a.png` | 不属于 `/blog/xxx/` |
| `/love/photos/a.jpg` | 穿透至 love 项目的对应目录 |
| 匹配资源前缀但文件不存在 | 资源 404，不回退到 SPA 的 index.html |
| 停用的资源前缀 | 保留拒绝规则，默认 404，不重新交给网站兜底 |

配置层只支持字面路径前缀，v0.1 不开放任意正则、Nginx 原文或任意重写语句。资源前缀必须以 `/` 结束，必须比 `projectBase` 更深，不能是整个 `/blog/`。同站点重复、相互包含的资源前缀首版直接拒绝，避免依赖不同组件的优先级差异。

不通过文件后缀决定“是否穿透”。只有先命中已登记资源路径，才进一步执行文件类型策略。文件后缀不能自动区分云端构建图片与本地素材。

### 5.3 路径与编码

配置前缀使用简单 ASCII URL 段；叶子文件名可有中文和空格，URL 按段编码。查询串不参与文件路径，但不能无条件删除可能用于业务授权的参数。

对点段、反斜杠、编码斜杠、重复编码、NUL、Windows ADS/设备名等建立统一拒绝规则及跨层测试。不要多次 URL decode，也不能仅用字符串 startsWith 校验本地目录。JSON Schema 只做字段结构校验，不替代路径语义、ACL 或请求时检查。

对同前缀大小写差异、浏览器路径正规化、Nginx/frp/Caddy 解码差异，必须通过端到端用例后才认定兼容。Windows 文件系统行为不能由 Linux 测试推断。

## 6. 云端 Nginx 集成

### 6.1 每个授权站点仅接入一个受管 include

示意：在明确选定的 `server` 块内加入：

```nginx
include /etc/yandu/nginx/current/main.locations.conf;
```

原来的网站页面、API 和证书配置继续保留。Yandu 只生成自己的 location 片段，不接管 nginx.conf、任意 server 块或原网站部署。

活动资源路由示意：

```nginx
location ^~ /blog/xxx/ {
    proxy_set_header Host $host;
    proxy_set_header Cookie "";
    proxy_set_header Authorization ""; # 公开资源模板；受保护模板需独立生成
    proxy_hide_header Set-Cookie;
    proxy_pass http://127.0.0.1:18080;
    proxy_cache off;
    proxy_buffering off;
}
```

此片段用于说明路径，不包含生产模板全部校验和错误处理。`proxy_pass` 不附带 URI，且没有额外 rewrite 时保留原请求 URI；不是把路径尾部的 `/` 随意增删。[S2]

公开资源去掉网站 Cookie/Authorization，避免把站点登录凭据无必要地送到本地文件服务；有密码保护的资源使用独立策略，不能仍套用清空 Authorization 的模板。业务登录接入另做云端授权层。

### 6.2 暂存、校验、重载与确认

云端助手进行站点级串行加锁；读取根拥有的授权注册表；校验清单；渲染固定模板到新 generation 目录；保存 journal 和旧指针；原子切换受管指针；执行整套 Nginx 配置校验；成功才发起重载；失败还原磁盘指针；重载后用站点 Host 与资源探针核验实际生效版本。

不能只依据 `nginx -s reload` 进程返回码就宣布“新规则已服务”。Nginx 可以重新载入配置并让旧 worker 优雅退出，但产品仍需检验目标 generation 与资源链路。[S3]

检测外部配置漂移、冲突 include、重复 location 或先执行的 rewrite 时，停止自动修改并给出差异。通用工具不能保证从任意 Nginx/SPA 配置推断出全部业务路由；安装阶段明确受支持配置形态与资源路径所有权。

### 6.3 停用不是立即释放路由

停用时保留同前缀的拒绝 location，防止资源请求掉回业务 SPA。只有用户执行独立的“释放路径给网站”操作，才删除这条保留记录。释放操作要显示影响范围，不与删除磁盘文件关联。

## 7. 受限 SSH 配置同步

### 7.1 为何选择这条实现路径

同域名自定义路径需要改变云端入口。不能声称只 reload frpc 就能完成。v0.1 复用云服务器已有 OpenSSH：Windows 用专用密钥建立受限会话，将 JSON 从标准输入发送给固定助手，结果由标准输出返回。免去新建一个公网管理 Web 服务。

OpenSSH 的 authorized_keys 支持固定 `command`，配合 `restrict` 可禁止 PTY、端口/代理/X11 转发等能力；只配置固定命令而不限制转发是不够的。[S6]

### 7.2 权限设计

专用账号只允许公钥认证；密钥绑定固定 principal/device；无交互 shell、无 SFTP、无远程端口转发、无任意命令。不能把 SSH_ORIGINAL_COMMAND 拼进 shell。主机密钥在配对时校验并固定，变化后阻断而不是自动信任。

真正需要特权的动作由根拥有、调用面很小的 helper 完成。通过受保护 Unix socket 或严格限制的固定 sudo helper 调用；helper 只读取有限 JSON、渲染固定目录下的受管配置并执行预定义 Nginx 检查/重载。账号不能写 helper、sudoers、授权注册表、原网站配置或证书。

设备身份由已认证的 SSH principal 决定，不信任清单自行声明。禁止传入 shell 命令、任意文件路径、Windows 目录、任意上游地址、Nginx 片段或额外管理监听端口。

### 7.3 清单协议

```json
{
  "schemaVersion": 1,
  "operationId": "550e8400-e29b-41d4-a716-446655440000",
  "siteId": "main",
  "expectedCloudRevision": 7,
  "sourceRevision": 12,
  "routes": [
    {
      "routeId": "blog-xxx",
      "projectId": "blog",
      "prefix": "/blog/xxx/",
      "state": "active",
      "accessPolicy": "public_read"
    }
  ]
}
```

此处不传 `rootPath`、SSH/DNS/root 凭据，也不传 proxy_pass 目标；云端从注册表确定域名和固定 frps 内部地址。清单是“该设备在该站点拥有的路由快照”，不能覆盖其他设备的条目。

幂等：同 operationId + 同内容返回同回执；同 operationId + 不同内容报冲突。expectedCloudRevision 用于乐观并发；断线重试先查询 operationId 结果，不盲目重复 reload。接收体限制、数量限制、字符白名单、站点/路径授权、请求超时及全局/站点锁为必需校验。

清单遗漏的旧路由转为停用保留，不自动释放。增加域名或扩大授权必须走明确的云端管理员操作，不可由普通资源修改越权完成。

### 7.4 撤销与失联

普通私有管理密钥可撤销；隧道认证单独轮换。首版为单一所有者可信设备，不将共享 frp token 描述为成熟的多租户设备授权。

当 Windows 离线时无法替它实际修改本地目录；云端可以紧急封禁入口，但不反向覆盖目录配置。下次上线显示封禁状态，不能自动恢复越过云端封禁。

## 8. 本地配置、状态机与故障恢复

SQLite 是本地目标配置和应用 journal 的唯一来源。组件配置位于受管生成目录；不支持手工编辑 generated 文件后反向覆盖数据库。frpc 采用配置文件 + reload，不同时启用 Store 修改同一批代理。

记录：本地目标 revision、本地实际 revision、资源路由 manifest digest、云端 ingress revision、隧道注册状态与外网检查结果。目录变更不影响路由 manifest 时不更新云端 Nginx。

### 8.1 新建项目的顺序

```text
保存草稿 → 用户明确启用 → 检查目录、ACL、路由
→ 校验候选 Caddy/frpc 配置 → 本地文件服务准备
→ frpc 注册路径 → 同步云端入口 → 外网探针验证
→ 标记实际生效
```

Caddy 提供配置加载 API；frpc reload 支持代理增删改，但非代理公共参数存在限制。[S7][S8] 这不是跨进程事务，不承诺多组件原子成功或当前视频连接绝对不受影响。

### 8.2 停用和权限收紧

先在本地对目标资源阻断，再同步云端拒绝规则及撤销代理。云端同步失败时保留本地阻断与待同步状态，禁止为了“恢复成功”回到公开旧权限。已被访客下载的文件不能召回。

### 8.3 状态展示

| 状态 | 解释 |
| --- | --- |
| 草稿/未发布 | 未授权公开 |
| 本地配置无效 | 路径、权限或配置错误 |
| 等待隧道 | 组件就绪但云端隧道未注册 |
| 等待入口同步 | frp 已注册，SSH/云端规则未成功 |
| 入口已应用、外网未验证 | DNS、TLS 或真实资源检查未通过 |
| 已生效 | 指定版本与资源探针检查通过 |
| 已停用、撤销待同步 | 本地已阻断，云端状态尚待收敛 |

探针核对随机内容、长度或摘要，不将网站通用 200 页面当成资源成功。探针在资源监听器内提供，不暴露管理健康 API、目录路径或密钥。处理过期探针和受保护资源的验证凭据。

崩溃恢复使用 journal、last-known-good、组件现状及云端回执核对。重启不能只读旧配置文件然后宣称达成目标。批量应用先校验并合并 reload，逐项显示结果。

## 9. Windows 管理体验

### 9.1 页面

| 页面 | 首版内容 |
| --- | --- |
| 总览 | Agent、文件服务、隧道、云端同步通道分别显示状态 |
| 项目 | 项目名、目录、网站路径、状态、复制配置、启停、删除映射 |
| 目录与文件 | 授权根目录树、分页列表、创建子目录、文件地址预览与复制 |
| 连接 | 导入连接包、服务器指纹、测试连接、轮换状态 |
| 操作记录 | 保存/应用/同步的阶段、错误、重试、脱敏诊断 |

主表单：项目名 + 选择目录 + 网站站点 + 页面基础路径 + 资源路径范围 + 是否启用。预览 URL 与实际 Windows 文件之间的映射。不在主表单展示端口和底层配置文本。

已有 URL 方案可直接填写 `/blog/xxx/`，不强制改名。新项目可建议 `/blog/media/` 这样的清晰范围，但不是全站强制标准。

### 9.2 后台与启动器

手动 `yandu ui` 或桌面快捷方式：启动/检查服务，等待本地健康接口，打开浏览器；重复执行不产生多套进程；关闭浏览器不停止服务。开机启动服务默认静默，云端断开不影响本地页面打开。

Windows 服务与用户桌面分离，浏览器由用户会话中的启动器打开；不是服务直接弹窗。[S10]

### 9.3 本机认证

管理页和底层 API 均仅监听回环地址。管理页仍需本地会话、Host/Origin/CSRF 校验和不执行外部资源的 CSP。启动器通过限定 ACL 的本机 IPC 取得短期票据并兑换会话；不把管理凭据写进 URL 查询日志。不给用户展示其他站点可利用的任意文件读写或 shell API。

回环不等于抵御已控制本机管理员的恶意程序；该威胁不作超出实际能力的保证。

## 10. 文件与同源安全

管理与资源分离到不同监听器、路由树和处理代码，不只以 `/admin` 目录名区分。frpc 的业务目标由程序固定为资源端口，用户不能填管理 API 端口。

本地根默认仅允许明确授权的 `D:/oss` 等目录，不授予 Everyone 整盘权限。服务只读身份与素材写入身份分开。v0.1 不承诺 UNC、网络映射盘或未解锁卷可用。

Caddy 的 root 不提供完整沙箱保证，符号链接可指向根外。发布目录需可信写入，检测 junction/reparse point、避免混放配置密钥，并执行最小文件权限；初次扫描不能防御所有并发链接替换。[S4]

**同域名不同路径不是不同浏览器安全源。** 本方案接受用户要求的同域路径，但不以 `/blog` 与 `/love` 作为强隔离边界。[S11]

因此首版默认 media_only，拒绝把未知 HTML、JS、可执行 SVG 等直接作为同源网页发布；不支持任意用户上传后直接内联执行。允许的媒体类型、响应 Content-Type、nosniff 和必要的附件下载策略一起控制。只添加 nosniff 不能把任意 HTML 变安全。[S12]

公网读取限 GET/HEAD；文件类型检查发生在资源路径命中之后；目录列表关闭。上传/删除/覆盖属于后续独立受控 API。媒体默认走流式响应，不整文件读入 Agent 内存；Range、HEAD、条件请求必须端到端验证。

公开资源不传站点 Cookie，云端抑制资源响应设置站点 Cookie；不修改原网站 cookie。密码保护是独立资源凭据，不等于网站登录状态。私密相册后续接业务授权或短期签名，不用“URL 难猜”代替鉴权。

缓存默认要求重验证，不开云端持久缓存；有内容哈希的不可变资源才用长缓存。缓存不是备份，私密资源不因缓存模板变成可共享。

## 11. 两端快速部署

以下命令是待实现接口，包中不提供同名可运行脚本。

### 11.1 已有 Nginx 网站

```bash
sudo bash ./install-cloud.sh \
  --mode existing-nginx \
  --site-id main \
  --domain example.com \
  --server-config /etc/nginx/sites-available/main.conf
```

安装器显示变更预览，备份，明确 server 块归属，再添加受管 include；安装 frps 与配置助手，配置服务，生成受限连接包。复用证书，不新增通配符 DNS 或证书要求。

检测到容器/面板托管、复杂嵌套、未知模板或文件权限冲突时停止自动改写，给出一次性接入片段和诊断。不停原网站、不无授权覆盖配置。

### 11.2 空服务器

```bash
sudo bash ./install-cloud.sh \
  --mode standalone \
  --site-id main \
  --domain example.com \
  --app-upstream http://127.0.0.1:3000
```

3000 是已有业务应用的示例端口，不由工具猜测或自动部署 blog。没有业务应用时可安装资源组件，但状态必须说明页面服务尚未接入。

安装 Nginx/frps/OpenSSH 必要能力及配置助手；有授权和网络条件时用 Certbot webroot 申请单域名证书，配置定时续期和经过校验的 reload hook。Certbot 支持只获取证书及 webroot 验证，不必停止已经运行的 Web 服务。[S13]

域名 DNS 和云安全组权限由用户提供；没有权限时列待办，不关防火墙或虚报 HTTPS 成功。默认公网 443、按证书方式使用 80、隧道 7000、已有 SSH 端口；frps HTTP 入口只监听 127.0.0.1:18080，Dashboard 关闭。

### 11.3 Windows

```powershell
.\install-windows.ps1 -ProfilePath .\main.yandu-profile
.\start.ps1
# 安装后日常也可运行
yandu ui
```

安装器解压预编译程序及 frpc/Caddy、建立受限状态目录、导入连接包、注册服务和快捷方式，再调用用户会话启动器打开页面。不要求运行时安装开发环境，不永久放宽 PowerShell 策略。

```text
C:\Program Files\Yandu\        程序与锁定版本组件
C:\ProgramData\Yandu\          数据库、生成配置、凭据、日志、备份
D:\oss\blog\                   用户资源，独立于工具生命周期
D:\oss\love\
```

安装应使用可重复执行的步骤记录；断点失败后可恢复，重复执行不重置凭据、不覆盖项目、不重复创建服务。Win 服务开机后磁盘未就绪显示等待并退避检查，不回退父目录。

## 12. 配对、凭据与信任

连接包含隧道地址、站点 ID/域名、SSH 主机密钥校验信息、专用设备认证材料、隧道 CA 等必要信息；不含云端 root 密钥、DNS 密钥、CA 私钥或网站证书私钥。

首版支持云端生成专用设备配对材料并以口令加密包交付，Windows 导入后受限 ACL/系统保护存储；云端生成时的临时私钥副本应有明确销毁流程。口令不放命令行和日志。后续可换本地生成密钥、公钥登记/CSR 流程，不混入首版公网注册平台。

隧道 TLS 必须明确验证 frps 身份，不能仅打开加密就认为已验证证书。frp 官方区分加密与证书验证。[S14] 凭据轮换与普通项目热更新分开处理，某些认证材料需要组件重连，不能宣传无感轮换。[S15]

## 13. 本地 API 与 CLI

仅本机 API，版本前缀 `/api/v1`。契约详见 `02_开发任务与接口契约.md`。

核心接口：项目列表与草稿创建、带 expectedRevision 的修改、校验、应用、停用、释放路由、受控目录列表/创建、连接包导入、操作状态、诊断导出。应用返回 operationId，不阻塞到浏览器连接超时。

核心 CLI：`ui`、`status`、`doctor`、`project add/list/apply/disable/remove`、`route release`、`config import/export`。UI 与 CLI 复用同一服务逻辑。导入只形成待确认变更，不自动公开全部资源。

错误至少区分 PATH_NOT_FOUND、PATH_OUTSIDE_ROOT、REPARSE_POINT_DENIED、ACCESS_DENIED、ROUTE_CONFLICT、PAGE_ROUTE_PROTECTED、URL_AMBIGUOUS、TUNNEL_OFFLINE、SSH_HOST_KEY_MISMATCH、CLOUD_AUTH_DENIED、NGINX_CONFIG_INVALID、INGRESS_VERIFY_FAILED、REVISION_CONFLICT。

## 14. 工程目录与模块边界

```text
yandu/
├── cmd/yandu/                 # Windows Agent、启动器、CLI
├── cmd/yandu-cloud/           # Linux 受限配置助手、安装诊断入口
├── internal/
│   ├── model/                 # 项目、站点、路由、版本
│   ├── pathpolicy/            # URL 与 Windows 目录规则
│   ├── compiler/              # 同一模型生成本地/云端派生配置
│   ├── reconcile/             # 应用、补偿、恢复、状态
│   ├── adapters/frp/
│   ├── adapters/caddy/
│   ├── adapters/nginx/
│   ├── cloudsync/             # SSH 协议、回执、幂等
│   ├── localapi/
│   ├── state/
│   ├── secrets/
│   └── service/               # SCM、进程生命周期、IPC
├── web/                       # React 本地界面
├── scripts/                   # 两端安装/升级/卸载/启动
├── schemas/                   # 本地配置和云端清单
├── tests/                     # 单元、Windows、联调、故障测试
├── deploy/                    # 服务、受管入口与证书模板
├── release/                   # 版本锁、构建、校验、许可
└── docs/
```

配置编译器为纯逻辑层，不直接执行 shell；执行层只调用固定二进制和受控参数。分离“生成正确配置”与“配置实际生效”，两者分别测试。

## 15. 开发阶段与门槛

| 阶段 | 交付 | 必须通过 |
| --- | --- | --- |
| M0 路径契约 | schema、映射与冲突校验 | 用户给定 URL 精确映射；页面不被接管 |
| M1 数据链路 | Nginx + frps/frpc + Windows Caddy | 真机两个目录；Range；管理页不可达 |
| M2 自动配置 | 受限 SSH 助手、版本/回执、Nginx 受管 include | 本地新增第三个路径，无人工云端操作 |
| M3 本地产品 | 页面、Agent、CLI、目录管理 | 无需编辑 Nginx/TOML/JSON；失败状态真实 |
| M4 部署恢复 | 安装器、后台、启动器、升级、卸载 | 干净系统无开发环境安装；重启/回退可用 |
| M5 交付验收 | 安全测试、故障注入、性能记录、离线包 | 全部必需用例及实际证据 |

先验证 M1 与 M2，不先做一个仍依赖手工配服务器的页面。版本号、下载地址、签名材料由正式构建确定，不在方案中虚构已存在发布。

## 16. 验收清单

| 编号 | 场景 | 通过条件 |
| --- | --- | --- |
| A01 | 同域页面/图片 | `/blog` 正常来自云端；示例图片正确来自 Windows |
| A02 | 页面 API | `/blog/api`、文章页、前端构建资源不被误穿透 |
| A03 | 前缀边界 | `/blog/xxxevil/` 不命中；冲突与嵌套规则被拒绝 |
| A04 | 路径转换 | 只去掉 `/blog` 一次；没有双 blog 或丢失 xxx |
| A05 | 不存在资源 | 返回资源 404，不返回 SPA HTML 200 |
| A06 | 第三个项目 | 在本地界面新增路径后自动同步 Nginx 与 frpc |
| A07 | 目录迁移 | URL 不变，目录可更新；不重载无变化的云端配置 |
| A08 | 管理隔离 | LAN/公网/IPv6 不能到本地管理及内部 API |
| A09 | SSH 限权 | 可同步；不能 shell、SFTP、转发或提交任意命令/路径 |
| A10 | 配置校验 | Nginx 错误保留旧运行配置，恢复磁盘指针并记录 |
| A11 | 幂等与并发 | 重试无重复副作用；冲突 revision 明确失败 |
| A12 | 停用/删除 | 本地立即阻断，云端失败不重公开；不删除磁盘文件 |
| A13 | 文件与账号 | 中文/空格、ACL、盘不可用、reparse/ADS 均有定义结果 |
| A14 | 媒体行为 | HEAD、合法/非法 Range、206、拖动、条件请求验证 |
| A15 | 同源风险 | 不直接执行未知 HTML/JS/SVG；不透传无必要站点 Cookie |
| A16 | 重启/关闭 | 关闭浏览器继续服务；开机恢复；不重复启动进程 |
| A17 | 通道失联 | SSH 断开不停止已有数据链路；frp 离线不影响页面主服务 |
| A18 | 实际状态 | 不能将网站通用 200 判为目标图片成功 |
| A19 | 生命周期 | 重装幂等；升级可恢复；卸载保留数据与他人网站 |
| A20 | 容量与扰动 | 建议 100 路由、10 项目并发，记录硬件网络及更新扰动 |

A20 是建议负载，不是实测能力。所有 Windows 路径和服务行为需真实 Windows 测试。仅有 schema 检查、Linux 单元测试或网页截图不能替代运行验收。

## 17. 发布与最终交付

正式软件交付物：源码、可重复构建说明、Windows 完整包、Linux 完整包、两端脚本、系统/DNS/入口支持矩阵、接口文档、运维和轮换指南、测试报告、依赖版本锁、校验/签名资料及第三方许可。

完整离线包解决组件下载，不意味着 DNS/HTTPS/隧道不需要网络。版本升级先备份数据库及配置，独立版本目录切换，健康检查失败回退；数据库不可逆迁移不能只换回 exe 就声称回退成功。

安装器只修改自己记录拥有的服务、配置、账号和规则；不永远降低脚本执行策略，不关安全软件，不删除业务文件或整个全局代理配置。

## 18. 运行边界与后续扩展

Windows 关机、休眠、断网或磁盘不可读时，本地资源没有源站；云端网页仍应正常，资源请求明确失败。工具不是备份或高可用存储；上行与云端带宽要测量，不能仅看硬盘容量。默认不改电源策略，常开需用户确认。

后续版本按真实需求增加云端 Caddy 适配、业务鉴权/签名 URL、本地上传流程、多设备调度、缓存或备份。不要因为项目数量增加就直接建设多租户平台、任意远程执行和双向配置覆盖。

最终交付判断：**本地选目录、登记现有网站资源前缀、点击应用后即可访问；页面继续走云端；管理界面始终只在 Windows 本机；新增路径无手工云端编辑；所有失败有可验证的状态与恢复路径。**

## 19. 官方技术依据

核对日期：2026-10-08。以下依据支撑组件能力，不表示 Yandu 已实现。

- [S1] frp URL 路由：`https://gofrp.org/en/docs/features/http-https/route/`
- [S2] Nginx proxy_pass URI 语义：`https://nginx.org/en/docs/http/ngx_http_proxy_module.html`
- [S3] Nginx 配置重载：`https://nginx.org/en/docs/control.html`
- [S4] Caddy file_server 与根目录边界：`https://caddyserver.com/docs/caddyfile/directives/file_server`
- [S5] Caddy uri strip_prefix：`https://caddyserver.com/docs/caddyfile/directives/uri`
- [S6] OpenSSH 固定命令和 restrict：`https://man.openbsd.org/sshd.8`
- [S7] Caddy 配置 API：`https://caddyserver.com/docs/api`
- [S8] frpc reload 与限制：`https://gofrp.org/en/docs/features/common/client/`
- [S9] Go embed：`https://pkg.go.dev/embed`
- [S10] Windows 服务与交互：`https://learn.microsoft.com/en-us/windows/win32/services/interactive-services`
- [S11] 同源定义：`https://developer.mozilla.org/en-US/docs/Web/Security/Defenses/Same-origin_policy`
- [S12] nosniff：`https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/X-Content-Type-Options`
- [S13] Certbot webroot / certonly：`https://eff-certbot.readthedocs.io/en/stable/using.html`
- [S14] frp TLS 身份校验：`https://gofrp.org/en/docs/features/common/network/network-tls/`
- [S15] frp 认证与材料重载限制：`https://gofrp.org/en/docs/features/common/authentication/`
