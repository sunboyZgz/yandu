"""Prepare cloud binaries for source installs and upgrades, without building the UI."""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess


def digest(path: Path) -> str:
    result = hashlib.sha256()
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            result.update(chunk)
    return result.hexdigest()


def build_cloud(root: Path, stage: Path, go: str) -> None:
    print("正在从源码编译云端程序（Go 最低版本由 go.mod 校验）…", flush=True)
    subprocess.run(
        [go, "build", "-trimpath", "-ldflags=-s -w", "-o", str(stage / "yandu-cloud"), "./cmd/yandu-cloud"],
        cwd=root, env={**os.environ, "GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0", "GOTOOLCHAIN": "local"},
        check=True,
    )


def prepare_frps(root: Path, stage: Path) -> None:
    print("正在检查 frps；优先复用已有组件，缺少时下载并校验…", flush=True)
    # Keep verified downloads across repeated runs, separate from deployed tools.json.
    config = root / ".local/cloud-install/tools.json"
    try:
        result = subprocess.check_output(
            [str(stage / "yandu-cloud"), "tools", "prepare", "--role", "cloud", "--component-root", str(root),
             "--config", str(config), "--json"], cwd=root, text=True,
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
        raise RuntimeError("frps 在准备期间发生变化，已停止准备；请重新执行。")
