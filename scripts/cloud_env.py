#!/usr/bin/env python3
"""Resolve a supported Go toolchain, installing a verified private copy if needed."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request


ROOT = Path(__file__).resolve().parent.parent
PREFIX = Path("/usr/local/lib/yandu-env")
METADATA_URL = "https://go.dev/dl/?mode=json&include=all"


def version_tuple(value: str) -> tuple[int, int, int]:
    if not re.fullmatch(r"\d+\.\d+(?:\.\d+)?", value):
        raise ValueError("无效的稳定版 Go 版本：" + value)
    fields = tuple(map(int, value.split(".")))
    return fields + (0,) * (3 - len(fields))


def minimum_go(root: Path = ROOT) -> str:
    match = re.search(r"^go\s+(\S+)\s*$", (root / "go.mod").read_text(), re.MULTILINE)
    if not match:
        raise RuntimeError("go.mod 缺少 Go 最低版本。")
    version_tuple(match[1])
    return match[1]


def probe_go(path: str, minimum: str) -> str | None:
    try:
        result = subprocess.run(
            [path, "version"], cwd="/", env={**os.environ, "GOTOOLCHAIN": "local"},
            text=True, capture_output=True, timeout=15, check=True,
        )
        match = re.fullmatch(r"go version go(\d+\.\d+(?:\.\d+)?) linux/amd64\s*", result.stdout)
        if match and version_tuple(match[1]) >= version_tuple(minimum):
            return match[1]
    except (OSError, subprocess.SubprocessError):
        pass
    return None


def find_go(minimum: str, prefix: Path = PREFIX) -> tuple[str, str] | None:
    candidates = [shutil.which("go"), "/usr/local/go/bin/go", str(prefix / "go/bin/go")]
    for path in dict.fromkeys(candidate for candidate in candidates if candidate):
        version = probe_go(path, minimum)
        if version:
            return path, version
    return None


def go_asset(minimum: str) -> dict:
    with urllib.request.urlopen(METADATA_URL, timeout=60) as response:
        releases = json.load(response)
    wanted = "go" + ".".join(map(str, version_tuple(minimum)))
    for release in releases:
        if release.get("version") != wanted:
            continue
        for asset in release["files"]:
            if (asset.get("os"), asset.get("arch"), asset.get("kind")) == ("linux", "amd64", "archive"):
                if asset.get("filename") != wanted + ".linux-amd64.tar.gz" or not re.fullmatch(r"[0-9a-f]{64}", asset.get("sha256", "")):
                    raise RuntimeError("Go 官方下载元数据无效。")
                return asset
    raise RuntimeError(f"Go 官方下载中未找到 {wanted} linux/amd64。")


def download_go(asset: dict, destination: Path) -> None:
    digest = hashlib.sha256()
    with urllib.request.urlopen("https://go.dev/dl/" + asset["filename"], timeout=60) as response, destination.open("wb") as output:
        while chunk := response.read(1024 * 1024):
            digest.update(chunk)
            output.write(chunk)
    if digest.hexdigest() != asset["sha256"]:
        raise RuntimeError("Go 下载文件 SHA256 校验失败，未安装。")


def extract_go(archive: Path, destination: Path) -> None:
    # Extract only regular files and directories within go/, on Python 3.10 too.
    with tarfile.open(archive, "r:gz") as source:
        for member in source:
            path = PurePosixPath(member.name)
            if path.is_absolute() or ".." in path.parts or not path.parts or path.parts[0] != "go" or not (member.isdir() or member.isfile()):
                raise RuntimeError("Go 归档包含不安全的路径或文件类型。")
            target = destination.joinpath(*path.parts)
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                with source.extractfile(member) as content, target.open("wb") as output:
                    shutil.copyfileobj(content, output)
                target.chmod(0o755 if member.mode & 0o111 else 0o644)


def ensure_go(minimum: str, prefix: Path = PREFIX, check: bool = False) -> tuple[str, str]:
    existing = find_go(minimum, prefix)
    if existing:
        return existing
    if check:
        raise RuntimeError(f"未找到 Go {minimum}+；运行 sudo bash ./setup-cloud-env.sh 补齐。")
    version = ".".join(map(str, version_tuple(minimum)))
    target = prefix / ("go-" + version)
    if target.exists() or target.is_symlink():
        # Recover a previously completed download whose link wasn't published.
        recovered = probe_go(str(target / "bin/go"), minimum)
        if not recovered:
            raise RuntimeError(f"已有无效工具目录 {target}，请检查后移走该目录再重试。")
    else:
        print(f"正在下载 Go {version}（官方归档与 SHA256）…", flush=True)
        asset = go_asset(minimum)
        prefix.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix=".prepare-", dir=prefix) as temporary:
            stage = Path(temporary)
            archive = stage / "go.tar.gz"
            download_go(asset, archive)
            extract_go(archive, stage)
            if not probe_go(str(stage / "go/bin/go"), minimum):
                raise RuntimeError("下载的 Go 无法运行或不满足最低版本，未安装。")
            (stage / "go").rename(target)
    link = prefix / "go"
    if link.exists() and not link.is_symlink():
        raise RuntimeError(f"{link} 已存在且不是工具链接，请检查后移走再重试。")
    with tempfile.TemporaryDirectory(prefix=".link-", dir=prefix) as temporary:
        staged_link = Path(temporary) / "go"
        staged_link.symlink_to(target.name)
        staged_link.replace(link)
    installed = probe_go(str(link / "bin/go"), minimum)
    if not installed:
        raise RuntimeError("私有 Go 安装后检查失败，请检查工具目录后重试。")
    return str(link / "bin/go"), installed


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args(argv)
    try:
        minimum = minimum_go()
        path, version = ensure_go(minimum, check=args.check)
        print(f"Python {sys.version.split()[0]}：{sys.executable}")
        print(f"Go {version}：{path}")
        print("Ubuntu 云端源码安装环境已就绪。")
        return 0
    except (OSError, ValueError, RuntimeError, tarfile.TarError, subprocess.SubprocessError) as error:
        print("环境准备失败：" + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
