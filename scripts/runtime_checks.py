"""Read-only host/runtime checks shared by source entry points and native builders."""
from __future__ import annotations

import os
import json
from pathlib import Path
import platform
import re
import shutil
import struct
import subprocess
import sys


def require_host(*, media: bool = False) -> str:
    system = {"win32": "windows", "darwin": "darwin", "linux": "linux"}.get(sys.platform)
    machine = platform.machine().lower()
    arch = {"amd64": "amd64", "x86_64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(machine)
    target = f"{system}-{arch}"
    if target not in {"windows-amd64", "darwin-arm64", "linux-amd64"} or struct.calcsize('P') != 8:
        raise RuntimeError(f"Unsupported host/interpreter: {sys.platform}/{machine}, {struct.calcsize('P') * 8}-bit Python. Use native Windows x64, macOS ARM64 or Linux x64 with 64-bit Python.")
    if system == "darwin" and tuple(map(int, platform.mac_ver()[0].split('.')[:2])) < (14, 0):
        raise RuntimeError("macOS 14+ is required; no dependencies were installed.")
    if system == "windows" and sys.getwindowsversion().build < 22000:
        raise RuntimeError("Windows 11+ (build 22000+) is required; no dependencies were installed.")
    minimum = (3, 12) if media else (3, 10)
    if sys.version_info < minimum:
        raise RuntimeError(f"Python {minimum[0]}.{minimum[1]}+ is required; selected {sys.version.split()[0]} at {sys.executable}. Select an existing compatible interpreter.")
    return target


def minimum_go(root: Path) -> str:
    match = re.search(r'^go\s+(\d+\.\d+(?:\.\d+)?)\s*$', (root / 'go.mod').read_text(), re.MULTILINE)
    if not match:
        raise RuntimeError('go.mod is missing a stable minimum Go version.')
    return match[1]


def version_tuple(version: str) -> tuple[int, int, int]:
    if not re.fullmatch(r'\d+\.\d+(?:\.\d+)?', version):
        raise ValueError('Invalid stable version: ' + version)
    fields = tuple(map(int, version.split('.')))
    return fields + (0,) * (3 - len(fields))


def find_go(minimum: str, target: str, extra: tuple[str, ...] = ()) -> tuple[str, str] | None:
    candidates = [shutil.which('go')]
    candidates += [shutil.which('go', path=directory) for directory in os.get_exec_path() if directory and directory != '.']
    candidates += list(extra)
    errors = []
    for path in dict.fromkeys(p for p in candidates if p):
        path = str(Path(path).absolute())
        if not Path(path).exists():
            continue
        try:
            result = subprocess.run([path, 'version'], cwd=Path(path).anchor,
                                    env={**os.environ, 'GOTOOLCHAIN': 'local'},
                                    text=True, capture_output=True, timeout=15, check=False)
            if result.returncode:
                raise RuntimeError(f"exit {result.returncode}: {(result.stderr or result.stdout).strip()}")
            match = re.fullmatch(r'go version go(\d+\.\d+(?:\.\d+)?(?:beta\d+|rc\d+)?) (\S+)\s*', result.stdout)
            if not match:
                raise RuntimeError('unrecognized version output: ' + result.stdout.strip())
            version, actual_target = match.groups()
            print(f'Go {version} ({actual_target}): {path}', file=sys.stderr)
            if re.fullmatch(r'\d+\.\d+(?:\.\d+)?', version) and version_tuple(version) >= version_tuple(minimum) and actual_target == target.replace('-', '/'):
                return path, version
            print(f'Go candidate rejected: requires stable {minimum}+ {target}; existing tool is preserved.', file=sys.stderr)
        except (OSError, subprocess.SubprocessError, RuntimeError) as error:
            message = f'Go detection failed at {path}: {error}'
            print(message, file=sys.stderr)
            errors.append(message)
    if errors:
        raise RuntimeError('Existing Go detection failed; it is not treated as missing. ' + '; '.join(errors))
    return None


def check_source_environment(root: Path, *, media: bool = False) -> str:
    target = require_host(media=media)
    print(f'Host {target}; Python {sys.version.split()[0]} (64-bit): {sys.executable}', file=sys.stderr)
    minimum = minimum_go(root)
    go = find_go(minimum, target)
    if not go:
        raise RuntimeError(f'Go {minimum}+ {target} is required. No system installer was called; select an existing tool or explicitly enable system installation in the platform setup script.')
    return go[0]


def require_build_tools(names: tuple[str, ...]) -> None:
    """Check build commands before downloads or creation of staging directories."""
    for name in names:
        path = shutil.which(name)
        if not path:
            raise RuntimeError(f'Missing native build tool: {name}. No system installer was called; supply existing build tools or a complete component bundle.')
        args = ['--version']
        if name == 'xcrun':
            selected = subprocess.run(['/usr/bin/xcode-select', '-p'], capture_output=True, text=True, timeout=15, check=False)
            if selected.returncode or not selected.stdout.strip() or not Path(selected.stdout.strip()).is_dir():
                raise RuntimeError('Apple developer tools are not configured. No installer was launched; prepare Xcode Command Line Tools explicitly or use a complete bundle.')
            args = ['--find', 'clang']
        if name == '7z':
            args = ['i']
        result = subprocess.run([path, *args], capture_output=True, text=True, timeout=15, check=False)
        if result.returncode:
            raise RuntimeError(f'Build tool detection failed for {path} (exit {result.returncode}): {(result.stderr or result.stdout).strip()}')
        if name == 'cmake':
            version = re.search(r'^cmake version (\d+\.\d+\.\d+)\s', result.stdout)
            if not version or version_tuple(version[1]) < (3, 16, 3):
                raise RuntimeError(f'CMake 3.16.3+ is required for the locked libheif source; got {result.stdout.strip()} at {path}.')
        print(f'Build tool {name}: {path}; {result.stdout.strip().splitlines()[0] if result.stdout.strip() else "probe passed"}', file=sys.stderr)


def require_windows_cpp() -> None:
    vswhere = Path(os.environ.get('ProgramFiles(x86)', 'C:/Program Files (x86)')) / 'Microsoft Visual Studio/Installer/vswhere.exe'
    if not vswhere.is_file():
        raise RuntimeError('Visual Studio 2022+ C++ x64 tools were not found. No installer was called; use existing tools or explicitly enable -AllowSystemInstall.')
    result = subprocess.run([str(vswhere), '-latest', '-products', '*', '-version', '[17.0,)', '-requires', 'Microsoft.VisualStudio.Component.VC.Tools.x86.x64', '-property', 'installationPath'],
                            capture_output=True, text=True, timeout=15, check=False)
    if result.returncode or not result.stdout.strip():
        raise RuntimeError(f'Visual Studio C++ x64 detection failed (exit {result.returncode}): {result.stderr.strip()}. No system installation was attempted.')
    print(f'Visual Studio C++ x64: {result.stdout.strip()}', file=sys.stderr)


def check_node_npm() -> list[str]:
    node_path = shutil.which('node')
    if not node_path:
        raise RuntimeError('Node 22.12.0+ with npm is required for cross-build/packaging. No system installer was called.')
    node = Path(node_path).resolve()
    result = subprocess.run([str(node), '-p', 'JSON.stringify({version:process.versions.node,platform:process.platform,arch:process.arch})'],
                            capture_output=True, text=True, timeout=15, check=False)
    if result.returncode:
        raise RuntimeError(f'Node detection failed at {node}: {(result.stderr or result.stdout).strip()}')
    try:
        info = json.loads(result.stdout)
        target = require_host()
        expected_os, expected_arch = target.split('-')
        expected_os = 'win32' if expected_os == 'windows' else expected_os
        expected_arch = 'x64' if expected_arch == 'amd64' else expected_arch
        stable = re.fullmatch(r'(\d+\.\d+\.\d+)(?:\+[0-9A-Za-z.-]+)?', info['version'])
        if not stable or version_tuple(stable[1]) < (22, 12, 0) or (info['platform'], info['arch']) != (expected_os, expected_arch):
            raise ValueError(f'requires Node 22.12.0+ {expected_os}/{expected_arch}')
    except (ValueError, KeyError, TypeError) as error:
        raise RuntimeError(f'Node metadata incompatible at {node}: {result.stdout.strip()}; {error}. Existing Node is preserved.') from error
    for entry in (node.parent / 'node_modules/npm/bin/npm-cli.js', node.parent.parent / 'lib/node_modules/npm/bin/npm-cli.js', node.parent.parent / 'libexec/lib/node_modules/npm/bin/npm-cli.js'):
        if not entry.is_file():
            continue
        probe = subprocess.run([str(node), str(entry), '--version'], capture_output=True, text=True, timeout=15, check=False)
        if probe.returncode or not re.fullmatch(r'\d+\.\d+\.\d+\s*', probe.stdout):
            raise RuntimeError(f'npm cannot run with selected Node: {entry}: {(probe.stderr or probe.stdout).strip()}')
        print(f'Node {info["version"]}: {node}; npm {probe.stdout.strip()}: {entry}', file=sys.stderr)
        return [str(node), str(entry)]
    raise RuntimeError(f'Selected Node has no bundled npm-cli.js: {node}. No global npm was installed.')
