#!/usr/bin/env python3
"""Prepare only the cloud binaries, then run the regular storage-only installer."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

from cloud_env import find_go, minimum_go


ROOT = Path(__file__).resolve().parent.parent


def go_tool() -> str:
    minimum = minimum_go(ROOT)
    found = find_go(minimum)
    if found:
        return found[0]
    raise RuntimeError(f"源码安装需要 Go {minimum}+；请先运行 sudo bash ./setup-cloud-env.sh。")


def digest(path: Path) -> str:
    result = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            result.update(chunk)
    return result.hexdigest()


def prepare(root: Path, stage: Path) -> None:
    cloud = stage / "yandu-cloud"
    print("正在从源码编译云端程序（Go 最低版本由 go.mod 校验）…", flush=True)
    subprocess.run(
        [go_tool(), "build", "-trimpath", "-ldflags=-s -w", "-o", str(cloud), "./cmd/yandu-cloud"],
        cwd=root, env={**os.environ, "GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0", "GOTOOLCHAIN": "local"},
        check=True,
    )
    print("正在检查 frps；优先复用已有组件，缺少时下载并校验…", flush=True)
    # Keep verified downloads and resolved paths across failed/repeated runs.
    # Only the compiled installer and terminal handoff use temporary staging.
    config = root / ".local/cloud-install/tools.json"
    try:
        result = subprocess.check_output(
            [str(cloud), "tools", "prepare", "--role", "cloud", "--component-root", str(root),
             "--config", str(config), "--json"],
            cwd=root, text=True,
        )
    except subprocess.CalledProcessError as error:
        raise RuntimeError(
            "frps 准备失败；检查上方下载地址的出站 HTTPS、DNS 或代理后重试。"
            "也可将匹配依赖锁的 Linux x64 frps 放到仓库 bin/frps，或使用完整 Linux 发布包。"
            f"已校验缓存保留在 {config.parent}。"
        ) from error
    tool = json.loads(result)["tools"]["frps"]
    binary = stage / "bin/frps"
    binary.parent.mkdir()
    shutil.copy2(tool["path"], binary)
    if digest(binary) != tool["sha256"]:
        raise RuntimeError("frps 在准备期间发生变化，已停止安装；请重新执行。")
    for name in ("install-cloud.sh", "install-cloud-storage.sh"):
        shutil.copy2(root / "scripts" / name, stage / name)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", required=True)
    parser.add_argument("--device", default="owner")
    args = parser.parse_args(argv)
    if sys.version_info < (3, 10):
        print("源码安装需要 Python 3.10+。", file=sys.stderr)
        return 1
    try:
        with tempfile.TemporaryDirectory(prefix="yandu-cloud-install-") as temporary:
            stage = Path(temporary)
            prepare(ROOT, stage)
            print("云端组件已准备完成，开始安装服务并生成连接包…", flush=True)
            # Keep stdin/stdout attached to the terminal for the passphrase prompt.
            try:
                subprocess.run(
                    ["bash", str(stage / "install-cloud.sh"), "--mode", "storage-only",
                     "--server", args.server, "--device", args.device], check=True,
                )
            except subprocess.CalledProcessError as error:
                raise RuntimeError(
                    f"云端安装阶段未完成（退出码 {error.returncode}），请按上方提示处理；"
                    "若服务已启用而连接包生成失败，可单独执行 pair 继续配对。"
                ) from error
        return 0
    except KeyboardInterrupt:
        return 130
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.CalledProcessError) as error:
        print("源码云端安装失败：" + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
