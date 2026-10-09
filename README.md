# 檐渡 · Yandu

把本地素材按指定路径接入现有网站。React 管理界面只在本机运行；Windows Caddy 读取磁盘，frp 建立经过证书验证的隧道，受限 SSH 助手自动维护云端 Nginx 的资源入口。

```text
https://example.com/blog                     → 原云端网站
https://example.com/blog/xxx/中文%20a.png     → D:/oss/blog/xxx/中文 a.png
```

项目从原目录的《檐渡_Yandu_落地实施方案_v0.1.md》实现。当前版本为 **0.1.0 验证版本**。源码、管理界面、两端程序和完整组件包均可构建；Windows 真机服务、Ubuntu 安装和真实公网的验收范围见 [测试报告](docs/TEST_REPORT.md)。未提供服务器和域名的情况下，不会配置或发布到你的真实云端。

## 当前功能

- 中文本机管理页：总览、项目、目录与文件、连接、操作记录。
- 多项目、多资源前缀、草稿、明确发布确认、停用、删除映射、独立释放旧路径。
- SQLite 目标配置与恢复日志、项目版本冲突检测、异步应用与失败重试。
- Caddy 逐请求调用独立 Go 安全网关，再由 Caddy `file_server` 读取文件。管理 API 不在资源监听器上。
- ASCII 配置路径、中文/空格文件名、路径边界、Windows ADS/设备名、链接与 reparse point 检查。
- 默认媒体白名单，拒绝 HTML/JS/SVG 和目录列表；只读 GET/HEAD，流式文件、Range 和条件请求。
- AES-256-GCM + scrypt 加密连接包、SSH 主机指纹固定、frps CA 与服务器名称验证。
- 云端声明式配置、设备/路径授权、幂等回执、串行加锁、Nginx 校验、指针回退、实际入口版本核验。
- 基础密码保护、独立资源凭据及手动输入密码的外网核验。
- Windows Service、用户会话启动器、两端安装/升级/卸载脚本、Windows/Linux/macOS 完整离线包。

## 在当前 macOS 上运行

开发环境：Go 1.26.2、Node.js 22、Python 3（仅构建/打包用）。运行最终客户端不需要这些开发工具。

```bash
make dev
```

它构建嵌入界面的本机客户端，使用仓库内 `.yandu/` 保存状态，并打开带一次性会话票据的浏览器。首次运行没有服务器连接和已发布项目。关闭浏览器不会停止 Agent。

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" status
.local/bin/yandu --state-dir "$PWD/.yandu" doctor
```

管理监听：`127.0.0.1:18765`。通过 `yandu ui` 打开页面；裸地址没有授权会话时会显示本机启动提示。票据在 URL fragment 中传递并立即删除，不进入 HTTP 请求或访问日志。

前台调试使用 `make dev-serve`，仅构建使用 `make dev-build`。已有后台 Agent 时应先停止该进程，再进入前台调试或加载新编译的代码；当前独立 React Vite 页面不能完成管理 API 的鉴权。

配对、macOS 素材目录、项目操作与重启的完整流程见 [开发模式使用](docs-site/guide/development.md)。Makefile 默认程序与状态目录分别为 `.local/bin/`、`.yandu/`，也可通过 `DEV_BIN_DIR` / `DEV_STATE_DIR` 覆盖；Agent 的监听端口仍固定。

## 部署

完整包位于 `release/artifacts/`：

- `yandu-0.1.0-windows-amd64.zip`：客户端、Caddy、frpc、PowerShell 脚本。
- `yandu-0.1.0-linux-amd64.tar.gz`：Agent CLI、云端助手、frps/frpc、Bash 脚本。
- `yandu-0.1.0-darwin-arm64.tar.gz`：当前开发机的本地运行包。

生产目标基线：Windows 11 x64 + Ubuntu 24.04 LTS x64 + 宿主机 Nginx。macOS 是开发与验证环境。包内均含依赖锁、第三方许可及 SHA256SUMS；这次构建没有 Authenticode、发行签名或公证。

先安装云端，再将加密连接包安全交付给 Windows：

```bash
sudo bash ./install-cloud.sh \
  --mode existing-nginx --site-id main --domain example.com \
  --server-config /etc/nginx/sites-available/main.conf --allow /blog/
```

```powershell
.\install-windows.ps1 -ProfilePath .\main.yandu-profile
```

进入管理页，授权素材目录、添加项目、核对映射并确认应用。已有网站的 DNS、证书和业务部署继续使用原有配置。安装器只支持保守识别的原生站点结构，遇到复杂 rewrite、容器或面板应按部署文档接入一次 include。

完整步骤：[部署说明](docs/DEPLOYMENT.md)，[接口与 CLI](docs/API.md)，[恢复、卸载与轮换](docs/OPERATIONS.md)，[安全边界](docs/SECURITY.md)。

## 使用文档网站

中文 VitePress 文档站位于 `docs-site/`，包括安装指南、目录映射、项目发布、密码保护、故障排查、技术参考，以及当前发布包的下载与 SHA-256 校验。

```bash
make docs-dev              # 文档开发服务器：http://127.0.0.1:5174/
make docs-build            # 安装锁定依赖、构建静态产物并检查链接和下载包
make docs-preview          # 预览生产构建
```

静态网站产物位于 `docs-site/.vitepress/dist/`。子路径部署、内容来源和维护说明见 [文档站 README](docs-site/README.md)。

## 构建与验证

```bash
make check                 # Go 测试、vet、React 类型检查与生产构建
make test-race             # Go race 检查
make release               # 校验上游下载、交叉编译、打完整离线包
make integration           # 真实 Caddy/frp/Nginx 链路，需要提供 Nginx 二进制
```

`release/dependencies.lock.json` 锁定 Caddy 2.11.4、frp 0.71.0 的官方 release 资产及 SHA-256。Go 模块和 UI 依赖分别由 go.sum/package-lock.json 锁定。`make integration` 可以设置 `YANDU_TEST_NGINX=/absolute/path/nginx`；缺少真实组件时测试会明确跳过，不算联调通过。

原始方案引用的 `02_开发任务与接口契约.md` 未随目录提供；已实现接口详见新建的 `docs/API.md`，不假定存在其他输入文件。
