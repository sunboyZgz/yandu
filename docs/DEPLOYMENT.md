# 部署说明

## 前提与交付边界

自动安装目标是 Windows 11 x64、Ubuntu 24.04 x64、宿主机 Nginx。当前 macOS 上的跨平台构建与本机链路测试不能替代 Windows ACL/SCM、真实 OpenSSH 限权、Ubuntu 安装和公网 DNS/HTTPS 的验收。不要用“能交叉编译”作为生产认证。

素材保留在设备；关机、休眠、断网和磁盘不可读都会使资源不可用。页面网站仍在云端运行。

## 已有 Nginx 网站

1. 解压 Linux 完整包并校验 `SHA256SUMS`，确认站点已可通过 HTTPS 访问。
2. 预览一次性接入修改：

   ```bash
   sudo bash ./install-cloud.sh --mode existing-nginx \
     --site-id main --domain example.com \
     --server-config /etc/nginx/sites-available/main.conf \
     --allow /blog/ --dry-run
   ```

3. 去掉 `--dry-run` 执行安装。安装器备份明确选定的站点文件，添加单个受管 include，执行 `nginx -t` 后 reload。不会改写全局 nginx.conf 或其他 server。
4. 安装时通过终端输入连接包口令，至少 12 字符。生成专用设备 SSH 私钥时只把加密包写盘，不产生明文临时私钥文件。
5. 安全传送 `main.yandu-profile` 到 Windows，另一个通道交付口令。导入后删除传输副本，妥善管理离线恢复备份。
6. 放行实际 SSH 端口和 frps TCP 7000。frps HTTP 18080 只绑定回环，Dashboard 不开启。不开放 Windows 18765/18766/18767/18768/18769 到 LAN。

`--allow /blog/` 授权该页面下更深的资源范围；不能发布整个 `/blog/`。`/blog/api/`、`assets/`、`posts/`、`_next/` 被保护。需要 `/love/`、`/gallery/` 等新页面范围时，由云端管理员明确向 `/etc/yandu/registry.json` 的 `allowedPrefixes` 添加范围，并通过配对材料轮换/导入更新本地授权。已授权页面内新增任意资源前缀无需 SSH 手工改配置。

站点注册表、设备身份和 helper 均由 root 拥有。`yandu-sync` 的公钥文件为 root 拥有，只允许固定 sudo helper，禁止 PTY、shell、SFTP、转发、agent/X11。配置助手不执行 SSH_ORIGINAL_COMMAND。

复杂配置不能自动接入时：在明确的 HTTPS server 块中加入以下片段，并排除先于 location 的 server 级 rewrite/return，再校验 reload：

```nginx
include /etc/yandu/nginx/current/main.locations.conf;
```

未知模板、容器或面板管理配置不标为自动支持。站点中已有 `^~` 或更深 location 与新资源范围冲突时，Nginx 全量校验/入口探针会报告失败；工具不宣称能推断所有业务路径。

## 空服务器

```bash
sudo bash ./install-cloud.sh --mode standalone \
  --site-id main --domain example.com --allow /blog/ \
  --app-upstream http://127.0.0.1:3000
```

必须明确提供自己的业务应用。安装器不部署博客，不猜测上游。独立模式创建 Nginx 站点，用 Certbot webroot 获取单域名证书，并设置校验后的 reload hook。DNS 需先指向服务器，80/443 对 ACME 可达。证书申请失败保留安装状态和诊断；不会虚报 HTTPS 完成或关闭防火墙。

## Windows

以管理员 PowerShell 执行完整包中的安装器。ExecutionPolicy Bypass 仅作用于这次进程，不改永久策略：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1 -ProfilePath .\main.yandu-profile
```

程序目录：`C:\Program Files\Yandu`。状态目录：`C:\ProgramData\Yandu`。业务素材例：`D:\oss\blog`。安装器配置状态 ACL，只允许安装用户、LocalService、Administrators、SYSTEM 访问。服务以 LocalService 身份运行；桌面快捷方式由用户启动浏览器。

需要给服务身份授予独立资源目录的读取权限，例如：

```powershell
icacls D:\oss\blog /grant '*S-1-5-19:(OI)(CI)RX'
```

目录创建功能另需该目录的写入权限。只读生产目录可以保留 RX，创建子目录时会明确报权限错误。不要授予 Everyone 整盘权限。管理页授权目录只代表产品许可，不会偷偷修改 Windows ACL。

创建项目示例：名称“个人博客”；标识 `blog`；站点 `main`；根 `D:/oss/blog`；基础路径 `/blog`；资源前缀 `/blog/xxx/`。保存后仍是草稿。应用确认展示实际范围；随后进行本地配置校验、Caddy 准备、frpc 注册、SSH 同步和 HTTPS 随机内容探针。

基础密码资源不会把明文密码保存到数据库；应用完成入口后，在连接页输入资源密码执行外网验证。网站登录 Cookie 不传入资源链路，资源密码与网站身份相互独立。

## 本机开发运行

`make dev` 在仓库 `.yandu` 启动本机 Agent，使用下载并校验的 Caddy/frpc。状态目录没有项目和连接材料时，文件服务只返回 404。真实部署需要你自己的站点、SSH 公钥指纹、CA 与设备材料；工程不内置真实服务器凭据。

Windows 服务与 Unix Agent 的健康接口只说明管理进程就绪。文件服务、隧道客户端和各项目注册/验证状态分别显示。缺失组件不影响进入管理页诊断，但不会被标为“已生效”。
