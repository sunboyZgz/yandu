#!/usr/bin/env python3
"""Native source development on Windows x64, Linux x64 and macOS ARM64."""
from __future__ import annotations

import argparse
import csv
import importlib.util
import io
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
from runtime_checks import require_host, check_source_environment, check_node_npm

ROOT = Path(__file__).resolve().parent.parent
TARGETS = {"windows-amd64", "linux-amd64", "darwin-arm64"}
PORTS = (18765, 18766, 18767, 18768, 18769, 18770)


class DevError(RuntimeError):
    pass


def host_target() -> str:
    return require_host()


def component_assets(root: Path, target: str) -> list[tuple[str, dict]]:
    system, arch = target.split("-")
    lock = json.loads((root / "release/dependencies.lock.json").read_text(encoding="utf-8"))
    extension = ".zip" if system == "windows" else ".tar.gz"
    caddy_os = "mac" if system == "darwin" else system
    names = [f"caddy_{lock['caddy']['version']}_{caddy_os}_{arch}{extension}",
             f"frp_{lock['frp']['version']}_{system}_{arch}{extension}"]
    try:
        return [(names[0], lock["caddy"]["assets"][names[0]]), (names[1], lock["frp"]["assets"][names[1]])]
    except KeyError as error:
        raise DevError(f"No locked components for {target}") from error


def executable_name(target: str) -> str:
    return "yandu.exe" if target.startswith("windows-") else "yandu"


def tool(name: str) -> str:
    found = shutil.which(name)
    if not found:
        raise DevError(f"Missing {name}. Install the development prerequisites and reopen the terminal.")
    return found


def npm_command(*args: str) -> list[str]:
    # Run the checked npm JavaScript entry directly, avoiding shell wrappers.
    return [*check_node_npm(), *args]


def execute(args: list[str], root: Path, **kwargs) -> subprocess.CompletedProcess:
    # Argument arrays and shell=False also preserve spaces and shell metacharacters in paths.
    return subprocess.run(args, cwd=root, check=True, **kwargs)


def check_output_files(bindir: Path, target: str) -> None:
    if sys.platform != "win32":
        return
    for path in (bindir / executable_name(target), bindir / "bin/caddy.exe", bindir / "bin/frpc.exe"):
        if path.exists():
            try:
                # No truncation: detect Windows executable write locks before installing any file.
                with path.open("r+b"):
                    pass
            except OSError as error:
                raise DevError(f"Development binary is locked or not writable: {path}. Stop the source Agent with Ctrl+C first.") from error


def npm_for_node(node: Path) -> list[str]:
    for entry in (node.parent / 'node_modules/npm/bin/npm-cli.js', node.parent.parent / 'lib/node_modules/npm/bin/npm-cli.js', node.parent.parent / 'libexec/lib/node_modules/npm/bin/npm-cli.js'):
        if entry.is_file(): return [str(node), str(entry.resolve())]
    raise DevError('Selected Node has no bundled npm-cli.js')


def ensure_npm(root: Path, folder: str, node: Path) -> None:
    directory=root/folder
    digest=hashlib.sha256((directory/'package-lock.json').read_bytes()).hexdigest()
    stamp=root/'.local/npm-stamps'/(folder+'.sha256')
    env={**os.environ,'PATH':str(node.parent)+os.pathsep+os.environ.get('PATH','')}
    modules=['typescript','vite'] if folder=='web' else ['sharp','exiftool-vendored']
    valid=False
    lockfile=directory/'node_modules/.package-lock.json'
    matches=False
    if lockfile.is_file():
        try:
            expected=json.loads((directory/'package-lock.json').read_text())['packages']
            installed=json.loads(lockfile.read_text())['packages']
            matches=all(name in expected and all(record.get(key)==expected[name].get(key) for key in ('version','integrity')) for name,record in installed.items()) and all(name=='' or record.get('optional') or name in installed for name,record in expected.items())
        except (KeyError,ValueError,OSError):matches=False
    if matches and (not stamp.is_file() or stamp.read_text()==digest):
        script=';'.join('await import('+json.dumps(name)+')' for name in modules)
        valid=subprocess.run([str(node),'--input-type=module','-e',script],cwd=directory,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0
    if not valid:
        execute([*npm_for_node(node),'--prefix',str(directory),'ci'],root,env=env)
    stamp.parent.mkdir(parents=True,exist_ok=True);stamp.write_text(digest)


def build(root: Path, bindir: Path, target: str, tools_config: Path | None = None, role: str = 'readonly', no_media: bool = False, overrides: dict | None = None) -> None:
    go = check_source_environment(root, media=role == 'local' and not no_media)
    build_env = {**os.environ, 'GOTOOLCHAIN': 'local', 'PATH': str(Path(go).parent) + os.pathsep + os.environ.get('PATH', '')}
    if target == host_target():
        check_output_files(bindir,target)
        bindir.mkdir(parents=True,exist_ok=True)
        config=tools_config or bindir.parent/'tools.json'
        with tempfile.TemporaryDirectory(prefix='.tool-bootstrap-',dir=bindir.parent) as temporary:
            helper=Path(temporary)/executable_name(target)
            execute([go,'build','-o',str(helper),'./cmd/yandu'],root,env=build_env)
            def prepare(config_path,selected_role,skip_media=False,explicit=None):
                args=[str(helper),'tools','prepare','--config',str(config_path),'--component-root',str(root),'--role',selected_role,'--json']
                if skip_media:args.append('--no-media')
                for name,path in (explicit or {}).items():args += ['--tool',name+'='+str(path)]
                print(f'Preparing {selected_role} dependencies...',flush=True)
                try:return json.loads(execute(args,root,stdout=subprocess.PIPE,env={**build_env,'YANDU_BUILD_PYTHON':sys.executable}).stdout)
                except subprocess.CalledProcessError as error:
                    detail=error.stderr.decode(errors='replace') if isinstance(error.stderr,bytes) else str(error.stderr or '')
                    raise DevError('Dependency preparation failed: '+(detail[-4000:] or 'see the component error above; verified download caches are kept.')) from error
            build_tools=prepare(config.parent/'build-tools.json','build')
            node=Path(build_tools['tools']['node']['path'])
            registered=prepare(config,role,no_media,overrides)
            ensure_npm(root,'web',node)
            if role=='local' and not no_media:ensure_npm(root,'media',node)
            env={**os.environ,'PATH':str(node.parent)+os.pathsep+os.environ.get('PATH','')}
            execute([*npm_for_node(node),'--prefix',str(root/'web'),'run','build'],root,env=env)
            execute([go,'build','-o',str(helper),'./cmd/yandu'],root,env=build_env)
            check_output_files(bindir,target)
            os.replace(helper,bindir/executable_name(target))
        print(f'Built {target}; dependency paths: {config}',flush=True)
        return
    # Cross-build checks cannot execute target binaries. Keep the locked archive
    # build flow, without attempting to register foreign tools as runnable.
    check_output_files(bindir, target)
    npm = npm_command()
    execute([*npm, "--prefix", str(root / "web"), "ci"], root)
    execute([*npm, "--prefix", str(root / "web"), "run", "build"], root)
    spec = importlib.util.spec_from_file_location("yandu_component_fetch", root / "release/fetch-components.py")
    if spec is None or spec.loader is None:
        raise DevError("Missing release/fetch-components.py")
    fetcher = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(fetcher)
    # Prepare only the two selected platform assets; fetch() verifies the committed SHA-256.
    components = [fetcher.fetch(asset) for asset in component_assets(root, target)]
    bindir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".dev-build-", dir=bindir.parent) as temporary:
        stage = Path(temporary)
        system, arch = target.split("-")
        env = {**build_env, "GOOS": system, "GOARCH": arch, "CGO_ENABLED": "0"}
        execute([go, "build", "-o", str(stage / executable_name(target)), "./cmd/yandu"], root, env=env)
        suffix = ".exe" if system == "windows" else ""
        for directory, name in ((components[0], "caddy"), (components[1], "frpc")):
            source = directory / (name + suffix)
            shutil.copy2(source, stage / (name + suffix))
        check_output_files(bindir, target)
        (bindir / "bin").mkdir(exist_ok=True)
        # Replacement is atomic per file; running Unix binaries retain their old inode.
        os.replace(stage / executable_name(target), bindir / executable_name(target))
        for name in ("caddy", "frpc"):
            os.replace(stage / (name + suffix), bindir / "bin" / (name + suffix))
    print(f"Built {target}: {bindir / executable_name(target)}", flush=True)


def require_native(target: str) -> None:
    if target != host_target():
        raise DevError(f"Cannot run {target} on {host_target()}. Use build or plan for cross-compilation.")


def ensure_ports_free() -> None:
    for port in PORTS:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
            try:
                probe.bind(("127.0.0.1", port))
            except OSError as error:
                raise DevError(f"Port {port} is in use. Stop the existing Agent/service before source development; it will not be stopped automatically.") from error


def prepare_state(root: Path, state: Path) -> None:
    if state == root or state == root / "web" or state == root / "docs-site":
        raise DevError("Choose a dedicated state directory, for example .yandu.")
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    if sys.platform == "win32":
        user = execute([tool("whoami"), "/user", "/fo", "csv", "/nh"], root, capture_output=True)
        raw = user.stdout
        encoding = "utf-16" if raw.startswith((b'\xff\xfe', b'\xfe\xff')) else ("utf-16-le" if b'\x00' in raw else "utf-8")
        rows = list(csv.reader(io.StringIO(raw.decode(encoding, errors="replace"))))
        sid = rows[0][1].strip() if rows and len(rows[0]) == 2 else ""
        if not re.fullmatch(r"S-1-(?:\d+-)+\d+", sid):
            raise DevError("Cannot resolve the current Windows user's SID.")
        execute([tool("icacls"), str(state), "/inheritance:r", "/grant:r",
                 "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F",
                 f"*{sid}:(OI)(CI)F"], root, stdout=subprocess.DEVNULL)
    else:
        state.chmod(0o700)


def client_args(binary: Path, state: Path, command: str) -> list[str]:
    args = [str(binary), "--state-dir", str(state), command]
    if command == "ui":
        args.append("--require-running")
    return args


def read_status(binary: Path, state: Path, root: Path) -> dict:
    result = execute(client_args(binary, state, "status"), root, capture_output=True, timeout=3)
    return json.loads(result.stdout)


def stop_child(child: subprocess.Popen, timeout: float = 15) -> None:
    if child.poll() is not None:
        return
    interrupt = getattr(signal, "CTRL_BREAK_EVENT", 1) if sys.platform == "win32" else signal.SIGINT
    child.send_signal(interrupt)
    try:
        child.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        # Only terminate the child created by this invocation; no PID scanning or service stops.
        child.terminate()
        child.wait(timeout=5)
        raise DevError("Agent did not exit gracefully. Check component processes before restarting.")


def run_agent(root: Path, bindir: Path, state: Path, target: str, no_open: bool, timeout: float, skip_build: bool = False) -> None:
    require_native(target)
    if not skip_build:
        check_source_environment(root)
    ensure_ports_free()
    if state == bindir or state.is_relative_to(bindir) or bindir.is_relative_to(state):
        raise DevError("Binary and state directories must be separate.")
    prepare_state(root, state)
    if not skip_build:build(root, bindir, target, tools_config=state/'tools.json')
    ensure_ports_free()
    binary = bindir / executable_name(target)
    options = {"creationflags": getattr(subprocess, "CREATE_NEW_PROCESS_GROUP", 0x200)} if sys.platform == "win32" else {"start_new_session": True}
    child = subprocess.Popen(client_args(binary, state, "serve"), cwd=root, **options)
    try:
        deadline = time.monotonic() + timeout
        ready = False
        while time.monotonic() < deadline:
            if child.poll() is not None:
                raise DevError(f"Agent exited during startup (code {child.returncode}).")
            try:
                status = read_status(binary, state, root)
                ready = bool(status["components"]["agent"] and status["components"]["fileService"])
            except (subprocess.SubprocessError, OSError, ValueError, KeyError):
                ready = False
            if ready:
                break
            time.sleep(0.2)
        if not ready:
            raise DevError(f"Agent/file service not ready after {timeout:g}s. Check {state / 'caddy.log'} and the terminal output.")
        print("Source Agent ready at http://127.0.0.1:18765/. Press Ctrl+C to stop.", flush=True)
        if not no_open:
            # The verified source Agent is already running: ui never needs Windows SCM startup.
            execute(client_args(binary, state, "ui"), root)
        returncode = child.wait()
        if returncode:
            raise DevError(f"Agent exited (code {returncode}).")
    finally:
        stop_child(child)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("run", "build", "ui", "status", "doctor", "plan"), nargs="?", default="run")
    parser.add_argument("--target", choices=sorted(TARGETS))
    parser.add_argument("--bin-dir", default=".local/bin")
    parser.add_argument("--state-dir", default=".yandu")
    parser.add_argument("--no-open", action="store_true")
    parser.add_argument("--timeout", type=float, default=30)
    args = parser.parse_args(argv)
    try:
        target = args.target or host_target()
        bindir = (ROOT / args.bin_dir).resolve()
        state = (ROOT / args.state_dir).resolve()
        binary = bindir / executable_name(target)
        if not 1 <= args.timeout <= 300:
            raise DevError("--timeout must be between 1 and 300 seconds.")
        if args.mode == "plan":
            print(json.dumps({"target": target, "binary": str(binary), "stateDir": str(state),
                              "assets": [name for name, _ in component_assets(ROOT, target)],
                              "agentArgs": client_args(binary, state, "serve"), "openBrowser": not args.no_open}, ensure_ascii=False, indent=2))
        elif args.mode == "build":
            build(ROOT, bindir, target, tools_config=state/'tools.json')
        elif args.mode == "run":
            run_agent(ROOT, bindir, state, target, args.no_open, args.timeout)
        else:
            require_native(target)
            if not binary.is_file():
                raise DevError("Source binary missing. Run the build mode first.")
            try:
                # Authenticate using this checkout's state before opening a page or diagnostics.
                read_status(binary, state, ROOT)
            except (subprocess.SubprocessError, OSError, ValueError) as error:
                raise DevError("This source Agent is not running/authenticated. Start run mode first and use the same state directory.") from error
            execute(client_args(binary, state, args.mode), ROOT)
        return 0
    except KeyboardInterrupt:
        print("\nSource development stopped.", file=sys.stderr)
        return 130
    except (RuntimeError, OSError, subprocess.SubprocessError, ValueError) as error:
        print(f"Development error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
