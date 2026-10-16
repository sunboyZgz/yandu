#!/usr/bin/env python3
"""Build selected cloud components from this checkout, then run the transactional upgrader."""
from __future__ import annotations

import argparse
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tempfile

from cloud_env import find_go, minimum_go
from cloud_source import build_cloud, prepare_frps
from runtime_checks import require_host


ROOT = Path(__file__).resolve().parent.parent
CONFIG = Path("/etc/yandu")
LIB = Path("/usr/local/lib/yandu")
UNITS = Path("/etc/systemd/system")


def drain_seconds(value: str) -> int:
    if not re.fullmatch(r"[0-9]{1,5}", value) or not 1 <= int(value) <= 86400:
        raise argparse.ArgumentTypeError("drain-timeout 必须为 1–86400 秒")
    return int(value)


def preflight(component: str) -> str:
    if require_host() != "linux-amd64":
        raise RuntimeError("源码云端升级需要在 Ubuntu 24.04+ x64 云服务器上执行。")
    release = platform.freedesktop_os_release()
    version = release.get("VERSION_ID", "")
    if release.get("ID") != "ubuntu" or not re.fullmatch(r"[0-9]{2}\.[0-9]{2}", version) or int(version.replace(".", "")) < 2404:
        raise RuntimeError("源码云端升级需要 Ubuntu 24.04+ x64。")
    if os.geteuid() != 0 or not any((CONFIG / name).is_file() for name in ("storage-owner.json", "registry.json")):
        raise RuntimeError("需要已安装的云端环境和 sudo；首次部署请使用 ./install-cloud.sh。")
    if not os.access(LIB / "yandu-cloud", os.X_OK):
        raise RuntimeError("已安装的云端程序不存在或不可执行，未开始编译。")
    if component == "gateway" and not all(path.is_file() for path in (CONFIG / "gateway.json", UNITS / "yandu-gateway.service")):
        raise RuntimeError("当前部署没有存储网关。")
    if component in ("tunnel", "all") and not (UNITS / "yandu-frps.service").is_file():
        raise RuntimeError("当前部署没有隧道服务。")
    for command in ("bash", "systemctl", "python3", "flock", "sha256sum", "mktemp", "install", "cp", "mv", "ln"):
        if not shutil.which(command):
            raise RuntimeError(f"缺少命令：{command}；未开始编译。请先准备云端环境。")
    manager = subprocess.run(["systemctl", "show", "--property=Version", "--value"],
                             text=True, capture_output=True, timeout=15, check=True)
    if not manager.stdout.strip():
        raise RuntimeError("systemd 管理器不可用，未开始编译。")
    minimum = minimum_go(ROOT)
    go = find_go(minimum)
    if not go:
        raise RuntimeError(f"源码升级需要 Go {minimum}+；可先运行 sudo bash ./setup-cloud-env.sh --allow-system-install 补齐环境。")
    return go[0]


def preview(component: str, drain: int) -> None:
    print(f"源码升级预览：{component}")
    print("正式执行时只编译云端 Go 程序，无需 Node/npm 或构建前端。")
    if component in ("cli", "all"):
        print("CLI：原子替换程序，不重启后台服务。")
    if component == "gateway" or component == "all" and (CONFIG / "storage-owner.json").is_file():
        print(f"网关：候选程序或配置变化且服务运行时切换，最多等待当前传输 {drain} 秒；新请求需重试。")
        print("旧网关首次切换可能中断传输，需安排维护窗口。")
    if component in ("tunnel", "all"):
        print("隧道：复用或下载并校验 frps；程序或配置变化且服务运行时需重连，正式执行需 --allow-disconnect。")
    print("本次仅检查环境和更新范围，不编译、不下载、不修改部署或操作服务。")
    print("尚未生成候选程序，是否需要重启须在正式构建后按内容和配置判断。")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="从源码根目录升级已安装的云端组件，默认只更新 CLI，不重启服务。")
    parser.add_argument("--component", choices=("cli", "gateway", "tunnel", "all"), default="cli")
    parser.add_argument("--drain-timeout", type=drain_seconds, default=300, metavar="秒数")
    parser.add_argument("--allow-disconnect", action="store_true", help="允许变化的活动隧道重连")
    parser.add_argument("--dry-run", action="store_true", help="只检查环境和更新范围，不编译或下载")
    args = parser.parse_args(argv)
    try:
        go = preflight(args.component)
        if args.dry_run:
            preview(args.component, args.drain_timeout)
            return 0
        with tempfile.TemporaryDirectory(prefix="yandu-cloud-upgrade-") as temporary:
            stage = Path(temporary)
            build_cloud(ROOT, stage, go)
            if args.component in ("tunnel", "all"):
                prepare_frps(ROOT, stage)
            shutil.copy2(ROOT / "scripts/upgrade-cloud.sh", stage / "upgrade-cloud.sh")
            command = ["bash", str(stage / "upgrade-cloud.sh"), "--component", args.component,
                       "--drain-timeout", str(args.drain_timeout)]
            if args.allow_disconnect:
                command.append("--allow-disconnect")
            print("源码组件已准备完成，开始校验并执行分组件升级…", flush=True)
            # The regular upgrader owns validation, locking, backups and rollback.
            result = subprocess.run(command)
            if result.returncode:
                print("升级未完成，请按上方提示处理；构建临时目录已清理，再次执行将重新准备候选程序。", file=sys.stderr)
            return result.returncode if result.returncode >= 0 else 128 - result.returncode
    except KeyboardInterrupt:
        return 130
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("源码云端升级失败：" + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
