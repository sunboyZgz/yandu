---
title: 准备云端网站
description: 在已有 Nginx 网站中接入资源路径，或为独立服务器配置业务上游。
---

# 准备云端网站

云端安装负责接入网站资源路径，不负责部署你的博客或业务应用。当前自动安装基线为 Ubuntu 24.04 x64 与宿主机原生 Nginx。

## 已有 Nginx 网站

在服务器上解压并进入 [Linux 发布包](../downloads.md) 目录，先确认实际网站的 HTTPS 地址可访问。

### 预览一次性接入

```bash
sudo bash ./install-cloud.sh \
  --mode existing-nginx \
  --site-id main \
  --domain example.com \
  --server-config /etc/nginx/sites-available/main.conf \
  --allow /blog/ \
  --dry-run
```

| 参数 | 含义 |
| --- | --- |
| `--site-id` | 本机项目使用的授权站点标识 |
| `--domain` | 已有网站的真实域名 |
| `--server-config` | 宿主机 Nginx 的实际站点配置文件 |
| `--allow` | 允许登记更深资源前缀的页面范围，例如 `/blog/` |
| `--dry-run` | 输出接入变更，不执行安装 |

自动接入需要唯一识别该域名的 HTTPS server 块，并排除先于资源 location 执行的 server 级 rewrite/return。

### 执行安装

核对预览后，用相同参数去掉 `--dry-run` 执行。安装器备份选定文件，添加受管 include，执行 Nginx 全量配置检查后 reload。

原网站页面、业务 API 和证书仍由原配置提供。后续在已授权范围内新增资源前缀由工具自动同步。

### 取得连接包

安装过程中通过终端输入至少 12 字符的口令。生成的 `main.yandu-profile` 包含设备连接材料，不包含 root、DNS 或网站证书私钥。

将加密包安全交付给 Windows，单独传递口令。导入完成后删除传输副本；恢复备份由你妥善保管。

## 新服务器或独立业务上游

已经有业务应用时，可以指定明确的上游：

```bash
sudo bash ./install-cloud.sh \
  --mode standalone \
  --site-id main \
  --domain example.com \
  --allow /blog/ \
  --app-upstream http://127.0.0.1:3000
```

`3000` 是示例端口，请替换为自己的应用端口。独立模式使用 Certbot webroot 申请单域名证书；DNS 应事先指向服务器，80/443 应满足申请条件。

## 核对网络条件

| 入口 | 用途 |
| --- | --- |
| HTTPS 443 | 网站与资源访问 |
| HTTP 80 | 按实际证书申请与续期方式使用 |
| 实际 SSH 端口 | 受限配置同步 |
| TCP 7000 | frp 资源隧道 |
| 回环 18080 | Nginx 到 frps 的内部入口，不开放公网 |

安装器不会关闭防火墙或代替你修改云安全组。组件包包含自研程序和 frp，系统软件的安装仍需要 apt 与网络条件。

## 自动接入停止时

容器、面板托管、复杂模板或配置冲突不在自动改写范围。依据 [部署契约](../reference/deployment.md)，由管理员在明确的 HTTPS server 块中接入一次受管 include，并检查、重载 Nginx。

需要扩大页面授权范围时，必须由云端管理员更新授权并重新交付配对材料。普通本机项目不能自行取得新域名或越过已有授权。
