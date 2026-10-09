# 安全边界与实现约束

管理端 18765、资源端 18766、安全网关 18767、frpc 内部 API 18768、Caddy 内部 API 18769 全部仅监听 IPv4 回环。frpc 的 localIP/localPort 固定为资源端，用户不能设置代理目标。Caddy 的资源路由树不包含管理页、目录 API、日志或配置端点。

管理端有精确 Host、Origin、CSRF、本机会话、单次启动票据和禁止外部资源的 CSP。状态目录需要 Unix 0700/0600 或 Windows 指定 ACL。凭据在本地 SQLite/文件内以 ACL 保护；当前没有额外 DPAPI 封装，不声称能抵御已控制本机的管理员。端口内部 API 并非公网认证服务。

资源检查由 Go 安全网关在每次 Caddy 文件请求前执行。它检查 Host、有效版本、发布状态、GET/HEAD、字面资源前缀、一次 URL 解码后的路径、扩展名和内容类型。Caddy 只去除一次 projectBase 并读取文件。云端/frp 不 rewrite URI。

禁止点段、反斜杠、编码斜杠、NUL、重复编码、ADS、Windows 设备名、末尾点/空格及非法字符。符号链接和 Windows reparse/junction 检查覆盖目标的各级祖先。公开目录必须由可信写入者维护：检查完成到 Caddy 打开文件之间仍有竞争窗口，不能把这套设计当作任意恶意写入者的完整文件沙箱。UNC/网络映射盘不在支持范围。

默认只允许 png/jpeg/gif/webp/avif/ico/mp4/webm/mp3/wav/ogg/flac。已知图片还做类型核验；媒体后缀下的 HTML/文本 payload 被拒绝。音视频不做完整容器解码，固定安全 Content-Type 并设置 nosniff。不会内联发布 HTML/JS/SVG、任意上传和目录列表。已有 blog 的文章/素材发布 CLI 仍须控制哪些文件被写入已授权公开范围；工具不会遍历其他目录替你发布文章。

公开资源在 Nginx 清空 Cookie/Authorization，隐藏 Set-Cookie；密码资源保留独立 Basic Authorization。网站登录态不转换为资源授权。基础密码使用 bcrypt，明文只在请求中短暂使用并不持久化；受保护探针需再次输入密码验证。不在日志写密码或资源内容。公开缓存要求重验证，密码资源 private/no-store。immutable 仅由用户选择用于内容哈希文件。

每个实际 Caddy 配置携带独立 generation；安全网关拒绝旧 generation。更新本地有效快照先于 reload，所以失败或旧配置进程不会按新授权继续读取旧根。停用和权限收紧先在数据库阻断，SSH 失败不会回到公开状态。已下载文件不能召回。

同域不同路径仍属同一浏览器源，不能隔离 `/blog` 和 `/love` 的主动内容。媒体白名单和可信目录写入是首版必要条件。

SSH 指纹必须匹配配对值，frpc TLS 必须验证 CA 和 serverName。共享 frp token 的方案仅面向单一可信所有者，不是多租户授权。云端注册表授权域名与路径，清单不能带新域名、Windows 根目录、shell 或任意上游。特权 helper 拒绝非 root 所有/可写的程序及注册表。

云端受管目录有完整内容漂移校验、全局 flock、版本检查、幂等摘要与落盘事务。Nginx 校验失败恢复受管指针；reload 返回并不等于验证通过，必须检查实际 generation。探针只返回随机无敏感内容，不暴露健康管理 API。
