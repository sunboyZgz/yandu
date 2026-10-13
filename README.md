# 檐渡 · Yandu

通用本地应用存储：文件保存在没有公网 IP 的本机，业务服务器经私有 TLS 隧道按需读写。支持虚拟目录、Tus 续传、版本、回收站、项目凭证和媒体任务；原有网站只读映射继续兼容。

```text
https://example.com/blog                     → 原云端网站
https://example.com/blog/xxx/中文%20a.png     → D:/oss/blog/xxx/中文 a.png
```

当前为 **v0.2 开发与验证版本**，目标平台是 macOS 14+ Apple Silicon、Windows 11 x64，云端 Ubuntu 24.04 x64。macOS 已运行原生 SFTPGo、HEIC/媒体与流式链路；Windows 真机、安装身份/生命周期和 Ubuntu 部署尚未完成验收，不能将交叉构建称为正式双平台验收。

新增入口与文档：

- [双平台存储开发、API 与 SDK](docs/STORAGE.md)
- [本机原生组件包与安装](docs/STORAGE_DEPLOYMENT.md)
- [Linux 云端服务器启动](docs/CLOUD_STARTUP.md)
- [一致性备份、恢复与迁移](docs/STORAGE_MAINTENANCE.md)
- [v0.2 实测结果与发布门槛](docs/STORAGE_VERIFICATION.md)
- [依赖检测与私有工具路径](docs/DEPENDENCY_TOOLS.md)
- [默认端口与监听配置](docs/PORTS.md)

```bash
python3 scripts/storage-dev.py --no-media  # 文件存储；完整媒体启动时去掉 --no-media
```

开发状态使用独立 `.yandu-storage-dev`，管理页 `127.0.0.1:18765`，存储 API `18770`，内部 SFTPGo `18771`。生成式服务端 SDK 在 `sdk/typescript`，业务会话代理示例在 `examples/storage-business`。完整客户端原生包使用 `release/build-native-storage.py`；传统 `release/build.py` 包不包含存储原生组件。

一键入口先验证 PATH 中已有工具，再检查 `tools.json` 路径和私有候选，只补齐缺失依赖并记录绝对路径、版本及 SHA256。不会修改系统 PATH 或安装全局 npm 包；已有正确组件和匹配锁文件的 npm 依赖会复用。完整包保留离线候选，源码缺少原生组件时才需要相应构建工具。

云端安装/升级提供 `sudo yandu list`、`sudo yandu disks`、`sudo yandu list --json`：通过独立只读监控凭证查看本机项目、容量及去重后的受管磁盘/APFS 容器。旧部署需更新两端程序和监控连接包，离线快照明确标注采样时间。步骤见[云端命令行与容量监控](docs-site/guide/cloud-monitoring.md)。

项目级 MP4 压缩支持固定字节上限、上传时明确的超限策略、原件完整保留、精确任务查询和实测产物摘要；用法见[媒体指南](docs-site/guide/media.md)，实际结果与待验收项见[压缩验收记录](docs/MEDIA_ACCEPTANCE.md)。

## 兼容的只读网站映射

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

开发环境：Go 1.26.2、Python 3；Node.js 22.23.1/npm 优先复用，缺少时准备 Yandu 私有副本。运行安装包不需要额外安装开发工具。

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

配对、macOS 素材目录、项目操作与重启的完整流程见 [只读映射开发](docs-site/guide/readonly-development.md)。Makefile 默认程序与状态目录分别为 `.local/bin/`、`.yandu/`，也可通过 `DEV_BIN_DIR` / `DEV_STATE_DIR` 覆盖；Agent 的监听端口仍固定。

## Windows 原生开发模式

Windows x64 安装 Go 1.26.2 和 Python 3.10+ 后，在源码根目录的普通 PowerShell 中执行；Node.js 22.23.1/npm 优先复用，缺少时自动准备：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev.ps1
```

脚本自动构建 React、校验并准备 Windows Caddy/frpc、编译 `yandu.exe`，以前台进程启动并打开管理页；按 Ctrl+C 停止。它使用当前用户和项目 `.yandu/` 状态，不注册系统服务。已占用的端口会明确报错，不自动停止安装版。

`-Mode build` 仅构建；第二个终端可用 `-Mode ui`、`status` 或 `doctor`。完整步骤见 [只读映射 Windows 开发](docs-site/guide/readonly-development-windows.md)。Windows 程序已交叉构建，启动逻辑与脚本入口经过验证；当前 macOS 环境不代表已完成 Windows 真机运行验收。

通用源码入口为 `python3 scripts/dev.py run`，支持 Windows x64、Linux x64、macOS ARM64；`build --target windows-amd64` 可交叉构建 Windows 开发程序。构建器只准备选定平台的资产，Go 编译与组件安装使用临时文件，避免在 Unix 上截断运行中的二进制。

## 部署

完整包位于 `release/artifacts/`：

- `yandu-0.1.0-windows-amd64.zip`：客户端、Caddy、frpc、PowerShell 脚本。
- `yandu-0.1.0-linux-amd64.tar.gz`：Agent CLI、云端助手、frps/frpc、Bash 脚本。
- `yandu-0.1.0-darwin-arm64.tar.gz`：当前开发机的本地运行包。

下列 v0.1 只读包的部署基线为 Windows 11 x64 + Ubuntu 24.04 x64 + Nginx。v0.2 通用存储的双平台安装见上面的新部署文档。包内均含依赖锁、第三方许可及 SHA256SUMS；这次构建没有 Authenticode、发行签名或公证。

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
