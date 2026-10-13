# 本机安装：macOS 与 Windows

## 构建完整客户端包

`release/build.py` 生成传统只读组件包，不能作为完整存储安装包使用。存储包必须包含 SFTPGo、Node、Sharp、ExifTool、FFmpeg、HEIC 解码器及依赖库、原始模板、许可材料和 `component-receipt.json`。

在目标平台原生构建。macOS 构建机先运行 `python3 scripts/build-native-codecs.py`；Windows 构建机使用 Visual Studio C++ Developer PowerShell、CMake、Git 和 7-Zip：

```bash
python3 release/prepare-native-components.py --output .local/native-components
python3 release/build-native-storage.py --components .local/native-components
```

Windows 使用 `python` 替代 `python3`。打包器检查工具版本，使用 npm 锁获取该平台的 Sharp/ExifTool，生成每个文件的 SHA256。构建结果在 `release/artifacts/native`。安装前和初始化时均验证组件收据；SHA256 表示完整性，发行身份还需单独签名。签名与平台验收状态见[验证记录](STORAGE_VERIFICATION.md)。

## macOS Apple Silicon

默认监听：管理页 `127.0.0.1:18765`、存储 API `127.0.0.1:18770`、SFTPGo 内部 API `127.0.0.1:18771`；兼容资源与组件管理使用回环 `18766–18769`。本机端口固定，配置来源见[端口配置](PORTS.md#本机开发与安装)。

解压原生包后，由需要管理文件的桌面账号运行：

```bash
sudo bash ./install-macos.sh
sudo bash /Library/Yandu/storage-root-macos.sh demo
```

返回默认根目录 `/Library/Application Support/Yandu/Storage/demo`。管理页填写相同 ID 和根目录。运行 `/Library/Yandu/start-macos.command` 打开管理页。

launchd 管理 `com.yandu.core` 和 `com.yandu.sftpgo`，分别使用 `_yandu` 与 `_yandusftpgo` 低权限身份。SFTPGo 写正式文件，核心服务管理数据库及暂存；桌面账号通过受限管理令牌和本机页面操作。正式文件、存储数据库、SFTPGo 配置和密钥分属受限目录。避免使用桌面、下载、照片库等个人隐私目录作为后台存储。

升级使用新包里的 `sudo bash ./upgrade-macos.sh`，停服务并执行一致性备份后更新组件、核验身份、重新设置目录权限。备份在 `/Library/YanduBackups`。`sudo bash ./uninstall-macos.sh` 停止并移除程序和 launchd 定义，保留状态、存储、备份与服务身份。

## Windows 11 x64

默认监听：管理页 `127.0.0.1:18765`、存储 API `127.0.0.1:18770`、SFTPGo 内部 API `127.0.0.1:18771`；兼容资源与组件管理使用回环 `18766–18769`。本机端口固定，配置来源见[端口配置](PORTS.md#本机开发与安装)。

在管理员 PowerShell 中进入完整存储包：

```powershell
.\install-windows.ps1
.\storage-root-windows.ps1 -ProjectId demo
.\start.ps1
```

默认安装 `Program Files\Yandu`，状态在 `ProgramData\Yandu`。SCM 分别运行 `Yandu` 和 `YanduSFTPGo`，使用独立虚拟服务身份 `NT SERVICE\Yandu`、`NT SERVICE\YanduSFTPGo`。安装器重新设置内部目录 NTFS ACL；只有管理令牌目录给予安装桌面账号读取权限。

项目根必须位于固定 NTFS 卷；`storage-root-windows.ps1 -ProjectId demo -BasePath 'D:\Yandu Storage'` 可准备其他固定卷。普通桌面账号经管理页操作文件，不直接写正式目录。

新包中运行 `.\upgrade-windows.ps1` 升级；备份在 `ProgramData\YanduBackups`。`.\uninstall-windows.ps1` 移除两个服务和程序，保留状态和存储。

云端须另行启动，步骤见[云端服务器启动](CLOUD_STARTUP.md)。完整链路顺序为云端、本机、配对、业务验证。
