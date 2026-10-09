---
title: 开发模式使用
description: 从源码在 macOS 启动檐渡，调试前台 Agent，使用本地目录、连接包与项目，并重新构建和停止开发实例。
---

# 开发模式使用

开发模式从源码构建并运行本机客户端。你可以在当前 macOS 上使用同一套项目、目录和连接功能，无需安装 Windows 客户端。

本页主流程适用于 **macOS Apple Silicon（ARM64）**。现有 Makefile 的组件复制路径针对该平台；Windows x64 和 Linux x64 的源码启动步骤见 [其他平台](#其他平台从源码启动)。

## 选择启动方式

| 命令 | 用途 | 进程行为 |
| --- | --- | --- |
| `make dev` | 快速构建并打开工具 | Agent 在后台运行，关闭浏览器或终端后继续运行 |
| `make dev-serve` | 开发与调试 | Agent 在当前终端运行，按 Ctrl+C 停止 |
| `make dev-build` | 仅构建 | 编译管理页、下载并校验组件、构建 Go 客户端，不启动 Agent |
| `make docs-dev` | 编辑本使用文档 | 启动 VitePress，端口为 5174 |

**工具管理页在 `127.0.0.1:18765`，使用文档在 `127.0.0.1:5174`。** 在文档页面中不能操作项目。

同一台机器只能运行一个使用当前固定端口的 Agent。若已经执行过 `make dev`，开始前台调试前先按 [停止后台实例](#停止后台实例) 核对并停止原进程。

## 准备开发环境

当前工程使用 Go 1.26.2、Node.js 22、npm 和 Python 3。在终端检查：

```bash
go version
node --version
npm --version
python3 --version
uname -m
```

本页主流程的 `uname -m` 应显示 `arm64`。进入实际源码目录；当前工作区可使用：

```bash
cd ~/Documents/my-projects/yandu
```

如果你把源码放在其他位置，请修改 `cd` 的路径。后文的 `$PWD` 均要求当前目录为项目根目录。

首次构建需要获取 npm/Go 依赖和 GitHub 上的组件。Caddy 2.11.4 与 frp 0.71.0 的版本及校验值来自仓库 `release/dependencies.lock.json`；组件下载脚本会准备锁定的多平台资产。

## 第一次启动并打开工具

```bash
make dev
```

该命令依次完成：

1. 安装 React 管理页的锁定依赖并构建静态资源。
2. 下载并校验 Caddy/frp，放置本机所需的 Caddy 和 frpc。
3. 构建 `.local/bin/yandu`，管理页嵌入该 Go 程序。
4. 使用项目 `.yandu/` 状态目录启动 Agent，并用一次性票据打开管理页。

**完成标志：** 浏览器显示檐渡总览，可以打开“项目管理”“服务器连接”和“操作记录”。命令行可读取状态：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" version
.local/bin/yandu --state-dir "$PWD/.yandu" status
```

再次打开管理页，执行：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" ui
```

直接访问裸地址没有会话时会显示启动提示。请用 `ui` 获取新票据，不需要从文件中手工复制本机令牌。

### 尚未准备服务器时

可以启动 Agent、查看界面与空状态、读取 `status` 和 `doctor`。没有连接包时没有授权站点，项目表单的保存按钮会不可用，CLI 创建项目也会报告 `CLOUD_AUTH_DENIED`。

因此，启动成功不等于可以创建并发布一个完整项目。实际项目操作需要你自己的云端连接包；源码目录不附带可用的公网测试账号或配对材料。

## 在前台调试 Agent

这适合修改 Go 代码、观察启动输出，以及明确控制进程生命周期。

**终端一：** 进入项目根目录，确认旧实例已经停止，再运行：

```bash
make dev-serve
```

看到“檐渡 Agent 已启动：127.0.0.1:18765”后，保持这个终端运行。

**终端二：** 同样进入项目根目录，打开管理页：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" ui
```

调试完毕，在终端一按 **Ctrl+C**。Agent 会关闭自己启动的 Caddy/frpc。项目记录与素材文件保留；后续启动仍会读取 `.yandu/` 中的配置。

## 开发模式下使用项目

### 1. 导入自己的连接包

云端仍需要真实的 Nginx 网站、域名和隧道服务。按 [准备云端网站](./cloud.md) 取得加密连接包。若希望云端也使用当前源码构建的程序，可以先执行 `make release`，再将生成的 Linux 包传到服务器部署。

在“服务器连接”选择连接包、输入口令并点击“导入并配对”；也可在项目根目录执行：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" connection import "$HOME/Downloads/main.yandu-profile"
```

将路径替换为实际文件位置。终端会隐藏输入连接包口令。随后在管理页点击“测试配置通道”，确认实际站点与 SSH 主机指纹。

### 2. 准备 macOS 素材目录

例如使用自己的 Home 目录：

```bash
mkdir -p "$HOME/yandu-assets/blog/xxx"
```

将一张真实 PNG 图片放到 `~/yandu-assets/blog/xxx/photo.png`。可以用 Finder 复制；该文件需要符合图片媒体策略，不能仅把 HTML 或 SVG 改名为 `.png`。

获取表单需要的绝对根目录：

```bash
printf '%s\n' "$HOME/yandu-assets/blog"
```

在管理页填写上面的完整输出，例如 `/Users/你的用户名/yandu-assets/blog`。表单不会展开 `$HOME` 或 `~`。Agent 以当前用户运行，目录需要允许当前用户读取；创建子目录还需要写入权限。

### 3. 创建草稿并浏览文件

在“项目管理”点击“新建项目”：

| 字段 | 开发模式示例 |
| --- | --- |
| 项目名称 | 开发博客 |
| 项目标识 | `blog-dev` |
| 本地素材目录 | 上一步输出的绝对目录 |
| 网站站点 | 从实际连接包的授权站点中选择 |
| 页面基础路径 | `/blog` |
| 资源路径范围 | `/blog/xxx/` |
| 访问权限 | 公开只读 |
| 缓存策略 | 每次重验证 |

示例要求站点授权包含 `/blog/`；若你的授权范围不同，请同步调整页面基础路径、资源前缀和素材子目录。若已有项目占用示例前缀，应选择授权范围内未被占用的前缀。

勾选素材目录授权后保存。项目状态为“未发布”，此时可以在“目录与文件”选择项目并浏览本地图片；复制的公网 URL 要等项目应用成功后才可用。

同样的草稿操作也可以通过 CLI 完成，以下示例要求连接包里的站点 ID 是 `main`：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" root grant "$HOME/yandu-assets/blog"
.local/bin/yandu --state-dir "$PWD/.yandu" project add \
  --id blog-dev --name 开发博客 --site main \
  --root "$HOME/yandu-assets/blog" --base /blog --prefix /blog/xxx/
.local/bin/yandu --state-dir "$PWD/.yandu" project list
```

页面与 CLI 是两种操作方式；如果已在页面创建同名项目，不要再重复执行 `project add`。

### 4. 明确应用并验证

在页面点击“应用”，核对目录和资源范围后点击“确认并应用”。CLI 的对应命令为：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" project apply blog-dev --confirm-publish
```

这一步会把素材接入你的真实网站；开发模式也遵守发布确认、媒体限制和权限策略。命令返回异步操作 ID，继续在“操作记录”查看进度，并通过 `status` 检查结果。

项目显示“已生效”后，用自己的实际域名验证：

```text
https://example.com/blog                 → 原云端页面
https://example.com/blog/xxx/photo.png   → macOS 本地图片
```

`example.com` 是占位域名。图片映射时只去掉一次 `/blog`，所以文件必须在素材根目录的 `xxx/photo.png`，而不是直接放在根目录。停用、密码项目和释放旧路径的流程与 [日常使用指南](./projects.md) 一致。

## 修改代码后如何生效

### Go 或 React 管理页

本版可实际使用的管理入口是 Go 内嵌管理页，**没有自动热重启**。前台模式按以下顺序更新：

1. 在 Agent 终端按 Ctrl+C。
2. 修改 `cmd/`、`internal/` 或 `web/src/` 中的代码。
3. 重新执行 `make dev-serve`；它先构建 React，再重新编译包含静态资源的 Go 程序。
4. 在另一个终端重新执行带相同状态目录的 `ui` 命令，获取新的本机会话。

后台模式先 [停止原 Agent](#停止后台实例)，再执行 `make dev`。仅在运行中的后台实例旁再次执行 `make dev`，会编译新程序并复用已运行的服务，**不能保证运行进程已加载新代码**。

### 为什么不能只启动 React Vite 页面

`npm --prefix web run dev` 默认提供 5173 端口的前端开发服务器，但当前 `/api` 代理没有适配 Agent 的精确 Host/Origin 校验与启动票据流程，会遇到 403 或启动提示，不能作为完整的项目管理入口。

请通过上面的“构建 React → 编译 Go → 重启 Agent → `ui`”流程验证界面功能。单独执行 `go run ./cmd/yandu` 也不会自动构建最新 React 资源，而且临时可执行文件目录未配套 Caddy/frpc 时可能报告 `COMPONENT_MISSING`。

### 只修改使用文档

```bash
make docs-dev
```

VitePress 会热更新 Markdown 和主题，地址为 `http://127.0.0.1:5174/`。它与运行中的 Agent 独立；如果已有文档服务器使用该端口，直接在现有服务器中查看更新。

## 停止后台实例

`make dev` 启动的 Agent 不随浏览器关闭而停止。macOS 上先查询管理端口和进程命令：

```bash
lsof -nP -iTCP:18765 -sTCP:LISTEN
```

将查到的 PID 代入下面的检查命令；`12345` 仅为示例：

```bash
ps -p 12345 -o pid,command
```

确认命令对应本项目 `.local/bin/yandu`、本项目 `.yandu` 状态目录及 `serve` 后，再使用该实际 PID：

```bash
kill -TERM 12345
```

随后可用 `make dev-serve` 转入前台模式，或用 `make dev` 重新启动后台实例。已有发布资源会在 Agent 停止期间暂时不可访问。

## 状态、日志和常见问题

所有开发 CLI 命令都应携带 `--state-dir "$PWD/.yandu"`。省略时会使用操作系统默认状态目录，而不是本项目状态；另一目录的令牌不能认证当前 Agent。

| 位置 | 用途 |
| --- | --- |
| `.local/bin/yandu` | 源码构建的客户端 |
| `.local/bin/bin/` | 配套的 Caddy 与 frpc |
| `.yandu/` | 数据库、凭据、生成配置、组件日志 |
| `.yandu/agent.log` | 后台启动的 Agent 输出；前台模式查看终端 |
| `~/yandu-assets/blog/` | 示例素材，独立于程序与状态 |

诊断命令：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" status
.local/bin/yandu --state-dir "$PWD/.yandu" doctor
```

| 现象 | 处理 |
| --- | --- |
| `COMPONENT_MISSING` | 使用 `make dev-build` 补齐二进制与组件；核对当前机器是 ARM64 |
| 端口已占用 / Agent 已在运行 | 先确认现有进程的状态目录，复用该实例或按上面的步骤停止 |
| 管理页提示从本机启动 | 重新执行携带本项目状态目录的 `ui` |
| 没有网站站点 / 无法保存项目 | 先导入自己的连接包，查看授权范围 |
| 路径不存在或不可读 | 使用实际绝对路径，确认文件存在、当前用户权限及无符号链接 |
| CLI 认证失败 | 确认当前目录、`--state-dir` 和正在运行的 Agent 一致 |
| 等待隧道 / 等待入口同步 | 按 [状态与故障排查](./troubleshooting.md) 检查真实通道 |

构建和重启会保留开发状态。需要备份或换用状态目录时，先停止 Agent；状态目录包含凭据，应与公开素材目录分开。

## 其他平台从源码启动

以下是已有锁定组件覆盖的 x64 平台源码步骤，不会注册 Windows 服务或部署云端系统服务。目标机器行为仍应结合 [验证报告](../reference/verification.md) 验收。

### Linux x64

在源码根目录准备 Go、Node.js/npm、Python 3 后执行：

```bash
make ui
python3 release/fetch-components.py
mkdir -p .local/bin/bin
go build -o .local/bin/yandu ./cmd/yandu
cp release/artifacts/deps/caddy_2.11.4_linux_amd64/caddy .local/bin/bin/
cp release/artifacts/deps/frp_0.71.0_linux_amd64/frpc .local/bin/bin/
.local/bin/yandu --state-dir "$PWD/.yandu" serve
```

保持前台终端运行，在另一终端同样进入源码根目录，执行携带 `.yandu` 状态目录的 `ui`。它用 `xdg-open` 打开浏览器，因此完整管理页体验需要桌面会话；无桌面的机器可使用 CLI。素材目录使用该机器上的实际绝对路径。

### Windows x64

在源码根目录的 PowerShell 中执行：

```powershell
npm --prefix web ci
npm --prefix web run build
python release/fetch-components.py
New-Item -ItemType Directory -Force .local/bin/bin | Out-Null
go build -o .local/bin/yandu.exe ./cmd/yandu
Copy-Item release/artifacts/deps/caddy_2.11.4_windows_amd64/caddy.exe .local/bin/bin/
Copy-Item release/artifacts/deps/frp_0.71.0_windows_amd64/frpc.exe .local/bin/bin/
$YanduDevState = Join-Path $PWD '.yandu'
& .local/bin/yandu.exe --state-dir $YanduDevState serve
```

第二个 PowerShell 窗口同样进入源码根目录，再执行：

```powershell
$YanduDevState = Join-Path $PWD '.yandu'
& .local/bin/yandu.exe --state-dir $YanduDevState ui
```

此时 Agent 以当前登录用户运行，素材权限按该用户检查。若安装包的 Yandu 服务已占用管理端口，应先停止该服务，再启动源码实例；二者不能同时使用当前固定端口。

## 验证代码修改

执行 Go 测试前先停止开发 Agent。真实组件用例可能由环境变量或 PATH 发现组件，会使用 18766–18769 等固定资源端口。

```bash
make check
make test-race
```

真实链路测试还需要真实 Nginx 二进制，再执行：

```bash
YANDU_TEST_NGINX=/absolute/path/nginx make integration
```

把 Nginx 路径替换为实际可执行文件；还应确保测试所需的回环 18080 可用。缺少组件时测试会明确跳过，不能当作真实链路验证成功。
