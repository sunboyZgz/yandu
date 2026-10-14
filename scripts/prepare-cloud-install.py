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


ROOT = Path(__file__).resolve().parent.parent


def go_tool() -> str:
    found = shutil.which("go")
    if found:
        return found
    # sudo's default PATH commonly omits the official Go installation.
    standard = Path("/usr/local/go/bin/go")
    if standard.is_file() and os.access(standard, os.X_OK):
        return str(standard)
    raise RuntimeError("源码安装需要 Go 1.26.2+；请安装 Go，或使用完整 Linux 云端发布包。")


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
        cwd=root, env={**os.environ, "GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0"},
        check=True,
    )
    print("正在检查 frps；优先复用已有组件，缺少时下载并校验…", flush=True)
    result = subprocess.check_output(
        [str(cloud), "tools", "prepare", "--role", "cloud", "--component-root", str(root),
         "--config", str(stage / "preparation-tools.json"), "--json"],
        cwd=root, text=True,
    )
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
            subprocess.run(
                ["bash", str(stage / "install-cloud.sh"), "--mode", "storage-only",
                 "--server", args.server, "--device", args.device], check=True,
            )
        return 0
    except KeyboardInterrupt:
        return 130
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.CalledProcessError) as error:
        print("源码云端安装失败：" + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
