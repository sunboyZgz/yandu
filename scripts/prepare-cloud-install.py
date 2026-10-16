#!/usr/bin/env python3
"""Prepare only the cloud binaries, then run the regular storage-only installer."""
from __future__ import annotations

import argparse
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

from cloud_env import find_go, minimum_go
from cloud_source import build_cloud, prepare_frps


ROOT = Path(__file__).resolve().parent.parent


def go_tool() -> str:
    minimum = minimum_go(ROOT)
    found = find_go(minimum)
    if found:
        return found[0]
    raise RuntimeError(f"源码安装需要 Go {minimum}+；请先运行 sudo bash ./setup-cloud-env.sh。")


def prepare(root: Path, stage: Path) -> None:
    build_cloud(root, stage, go_tool())
    prepare_frps(root, stage)
    for name in ("install-cloud.sh", "install-cloud-storage.sh"):
        shutil.copy2(root / "scripts" / name, stage / name)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", required=True)
    parser.add_argument("--device", default="owner")
    parser.add_argument("--allow-system-install", action="store_true")
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
                installer_args = ["--allow-system-install"] if args.allow_system_install else []
                subprocess.run(
                    ["bash", str(stage / "install-cloud.sh"), "--mode", "storage-only",
                     "--server", args.server, "--device", args.device, *installer_args], check=True,
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
