# v0.1.0 验证报告

日期：2026-10-08（Asia/Shanghai）。结论：已实现可运行的双端工程与本机产品；Go/React 构建、安全与状态测试、真实本机数据链路通过。**尚不属于 Windows + Ubuntu + 真实公网全部验收通过的正式发行版。**

## 已使用的真实环境

- macOS 15.6.1，MacBookPro18,3，arm64，8 CPU 核，16 GiB 内存。
- Go 1.26.2，Node 22.23.1。
- Caddy 2.11.4、frpc/frps 0.71.0 官方二进制，下载资产 SHA-256 已校验。
- Nginx 1.30.5 从官方源码在仓库 `.local/` 编译，未安装到系统 Nginx 目录。
- 真实 Nginx HTTP 分流、frp TLS 控制/数据链路、Caddy 文件读取、Go 安全网关；前端 HTTPS 由测试 TLS gateway 提供并由测试 CA 验证。
- SSH 客户端连接真实 Go SSH 测试服务，验证设备密钥、固定 principal 协议与主机指纹；没有把它冒充实际 Ubuntu OpenSSH/sudo 的系统验收。

## 已通过的检查

`npm --prefix web run build`：TypeScript 与生产构建成功；资源已嵌入 Go。

`go test -race -v ./...`：单元、协议、安全、版本/事务与真实组件测试通过；没有 race 报告。通常未指定组件时真实链路测试明确 skip，本次通过显式二进制路径实际执行。

`go vet ./...`、Bash 语法检查、Python 编译检查通过。Windows x64/Linux x64/darwin arm64 交叉编译成功。PowerShell/SCM/ACL 没有在 macOS 模拟成“已运行通过”。

真实链路用例覆盖：

- 三个项目自动建立 Nginx 和 frpc 路由，随机资源内容校验成功。
- `/blog`、`/blog/api`、文章页、构建图片及 `/blog/xxxevil/` 保留云端响应。
- 中文/空格文件名读取正确，文件字节逐一对比；去除一次 `/blog` 后仍保留 `xxx`。
- HEAD 无正文；合法 Range 206、越界 Range 416、ETag 条件请求 304。
- 缺失资源 404，不回退为 SPA 200；资源端 `/api/v1/status` 404。
- 公开资源不转发网站 Cookie/Bearer 凭据；密码资源保留独立 Basic Authorization。
- 公开改密码先阻断，密码挑战/正确密码读取及受保护探针成功。
- 删除映射保留 404；独立释放后原网站恢复处理对应路径。
- 目录迁移保持 URL，云端 revision 不增加。
- SSH 断开时已建立数据通道继续工作；停用立即阻断，失败仍保持阻断；业务文件没有删除。
- 错误 SSH 主机指纹在发送清单前拒绝。
- 云端幂等、内容冲突、旧 revision、路径/设备授权、配置漂移、Nginx 校验失败指针回退、管理员封禁。
- 路径穿越、重复编码、编码斜杠、NUL、ADS、设备名、符号链接、恶意媒体后缀 HTML、SVG、方法限制、本机会话/Host/Origin/CSRF、单次票据。
- 草稿不会公开；版本检查；批量配置导入失败整体回退；SQLite 重启保留已提交状态。

## 建议容量场景的实际记录

本机真实组件注册 **100 条路由 / 10 个项目**；10 个项目同时读取各自的 68 字节 PNG，并检查全部内容一致。

一次记录：配置扩大及逐项核验约 **8.27 秒**；10 个并发读取完成约 **35.73 毫秒**。这包含本机回环与 TLS gateway，图片非常小；它不代表 Windows 真机性能、视频吞吐、上行带宽、公网延迟或活动视频热更新的扰动。没有用这组数字承诺生产容量。

复现：

```bash
YANDU_TEST_CAPACITY=1 \
YANDU_TEST_CADDY=/absolute/path/caddy \
YANDU_TEST_FRPC=/absolute/path/frpc \
YANDU_TEST_FRPS=/absolute/path/frps \
YANDU_TEST_NGINX=/absolute/path/nginx \
go test -race -v ./...
```

## 浏览器检查

Playwright Chromium 实际完成：单次票据登录、连接包导入、目录授权、新建草稿、中文目录/文件列表、创建子目录、公开确认、异步应用/真实失联状态，以及桌面 1440×1050、移动 390×844 布局检查。浏览器发现的 HTML pattern `/v` 校验问题已修复；最终会话无控制台错误。截图保存在开发工作区 `output/playwright/`，不包含实际服务器秘密。

这轮浏览器使用的是临时测试材料，已从默认运行状态分离，发行包不附带测试连接包或已发布项目。

## 对照原方案 A01–A20

| 编号 | 本次证据 | 仍需目标环境验收 |
|---|---|---|
| A01–A07 | 真实本机三项目链路、分流、第三路径、缺失/迁移测试 | Windows 磁盘与真实站点 |
| A08 | 绑定配置、管理鉴权、资源端 API 不可达 | Windows LAN/IPv6 实际网络测试 |
| A09 | SSH 字节协议、身份/指纹、安装限权配置 | 实际 OpenSSH shell/SFTP/转发/sudo 拒绝测试 |
| A10–A12 | 回退/幂等/版本、停用、删除、释放通过 | Linux 崩溃注入及目标机器故障 |
| A13 | 中文/空格、路径/符号链接规则通过 | Windows ACL、junction/reparse、盘未就绪及大小写行为 |
| A14–A15 | 真实 Range/HEAD/ETag、媒体、Cookie/Authorization 通过 | 长视频拖动、真实业务 Authorization 联合测试 |
| A16 | Agent 进程锁、启动器、后台设计与恢复测试 | Windows 开机/SCM/桌面会话与强杀重启 |
| A17–A18 | 通道失联隔离及精确随机资源探针通过 | 公网 DNS/TLS/网络扰动 |
| A19 | 包、脚本、备份回退实现及语法/构建检查 | 干净 Windows/Ubuntu 安装、重装、升级、卸载 |
| A20 | 本机 100 路由/10 项目并发小文件测试 | Windows、公网大文件/视频、带宽和更新时间记录 |

## 已知运行限制

- 无 UNC/网络映射盘承诺；根目录写入者必须可信，Caddy 文件打开前存在链接替换竞争窗口。
- 凭据用状态目录 ACL 保护，未另作 DPAPI；不能抵御已控制本机的管理员。
- 受保护资源需要再次输入密码核验外网；不持久化明文密码。
- 有明确单一可信所有者边界，不提供多租户、业务登录、签名 URL、上传或任意代理。
- 云端安装仍需系统包、DNS/HTTPS 与网络。离线组件包包含 Caddy/frp/自研程序，不包含 Ubuntu 全套 apt 依赖。
- 发布包无 Authenticode、发行签名或 macOS 公证。
- 初期联调曾写入 Caddy 的共享 autosave；后续配置与进程已禁用自动保存、隔离到项目状态目录。相关本机恢复事项单独向用户说明，不视为正常安装行为。
