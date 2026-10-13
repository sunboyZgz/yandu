# 存储维护、备份和迁移

卷标识、APFS 容器及空闲空间在操作前检查。卷不可用或身份变化时操作停止；外部修改、符号链接、junction/reparse 或多硬链接异常会隔离项目。管理员排查后执行维护核验，不能直接修改 SQLite 清除隔离标记。

## 一致性备份

先停 Agent 和 SFTPGo，使文件、两个存储数据库和管理状态都保持静止。macOS 使用 `launchctl bootout system/com.yandu.core` 与 `launchctl bootout system/com.yandu.sftpgo`；Windows 使用 `Stop-Service Yandu,YanduSFTPGo`。开发模式按 Ctrl+C 停止拥有的进程。

```bash
/Library/Yandu/yandu --state-dir '/Library/Application Support/Yandu' \
  storage-maintenance --action backup --destination /absolute/empty/backup
```

安装状态操作需要管理员权限；上例命令加 `sudo`。Windows 等价：

```powershell
& 'C:\Program Files\Yandu\yandu.exe' --state-dir 'C:\ProgramData\Yandu' storage-maintenance --action backup --destination 'D:\Backup\Yandu-20261010'
```

备份目标必须为空且不与状态或任一项目根重叠。SQLite 用 `VACUUM INTO` 取得独立快照，文件复制后核对 SHA256；`backup.json` 保存项目原路径及所有文件校验值。备份包含密钥与原件，使用受限目录存放。容量足够后才开始，失败的备份不能作为恢复源。

## 恢复

停止两个服务，保留原状态及文件到独立位置，不覆盖现有目录。恢复要求新的状态目录为空，备份记录的每个原始根目录已存在且为空：

```bash
yandu --state-dir /absolute/empty/state storage-maintenance \
  --action restore --destination /absolute/complete/backup
```

恢复校验整个备份，保留项目、对象、版本、凭证和目录 ID，重新核对卷身份并更新文件基线；所有项目保持停用。恢复后重新运行完整包安装器/目录权限维护，使目录归属正确，再启动 SFTPGo，执行下方核验，并在管理页逐项启用。不要在两个目录中同时启动同一设备的副本。

恢复当前要求原始根路径保持一致；变更路径请先按迁移流程操作。恢复步骤会写入新状态，若中途失败保留失败副本排查，再从新的空目录重试，不跳过校验。

## 核验与解除隔离

Agent 停止、SFTPGo 运行时，管理员执行：

```bash
yandu --state-dir /absolute/state storage-maintenance --action verify --project demo
```

检查全部版本及派生物的链接、长度和 SHA256，成功后才解除隔离。修改或损坏文件必须从已校验备份恢复，再核验；正式文件经 SFTPGo 操作，不能直接覆盖后当作新版本。

## 维护迁移

Agent 停止，SFTPGo 保持运行以便使用官方用户 API 更新 home。准备同平台受支持文件系统上的空目录，执行：

```bash
yandu --state-dir /absolute/state storage-maintenance --action migrate \
  --project demo --destination /absolute/new-empty-root
```

复制并校验原件、版本、回收站和暂存，更新 SFTPGo home 与本地记录，保留对象 ID，项目保持停用；旧目录保留供回退。由于 SFTPGo 与 Yandu 各自拥有数据库，迁移期间不得启用服务，意外中断须核对两侧 home/root 是否一致后维护恢复，启动检查会拒绝不匹配的目录。

重新设置新根目录服务身份和权限，然后启动 Agent，在管理页确认目录、容量和历史版本再启用。开发目录和安装服务目录不能混用。macOS 使用独立 `_yandusftpgo` 写 `files`、`_yandu` 管理 `temporary`；Windows 使用相应两个服务 SID。迁移是管理员维护操作，不对业务凭证开放。

## 续传与任务恢复

Tus 的 `.info`、暂存及官方文件锁记录和意图数据库共同恢复上传。遗漏完成回调时服务根据偏移再次校验并提交；响应丢失使用原 Idempotency-Key 查询已有意图。版本提交采用条件比较，正式移动响应丢失时按目标内容校验恢复，不生成另一个版本。

Worker 租约过期后恢复到队列；失败最多尝试三次。未提交的派生物有持久清理记录，在 SFTPGo 恢复后清理，清理前保留预留空间。任务在写入结果和数据库提交前重查版本、删除和授权；已删除对象不会由任务重新出现。
