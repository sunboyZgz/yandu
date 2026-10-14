"""Verify cloud bootstrap policy and failure isolation without host installation."""
import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import cloud_env as env


class GoEnvironmentTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="yandu env & spaces ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.prefix = self.root / "private"

    def test_go_minimum_comes_from_module(self):
        (self.root / "go.mod").write_text("module example\n\ngo 1.27.3\n")
        self.assertEqual(env.minimum_go(self.root), "1.27.3")

    def test_version_admission_preserves_newer_stable_go(self):
        for value, accepted in (("1.26.2", True), ("1.26.9", True), ("1.27.1", True),
                                ("2.0.0", True), ("1.26.1", False), ("1.25.9", False),
                                ("1.27rc1", False)):
            with self.subTest(version=value), patch.object(env.subprocess, "run", return_value=
                    subprocess.CompletedProcess([], 0, f"go version go{value} linux/amd64\n", "")) as run:
                self.assertEqual(env.probe_go("/go", "1.26.2"), value if accepted else None)
                self.assertEqual(run.call_args.kwargs["env"]["GOTOOLCHAIN"], "local")
                self.assertEqual(run.call_args.kwargs["cwd"], "/")
        with patch.object(env.subprocess, "run", return_value=
                subprocess.CompletedProcess([], 0, "go version go1.27.1 linux/arm64\n", "")):
            self.assertIsNone(env.probe_go("/go", "1.26.2"))

    def test_older_path_go_falls_back_to_private_toolchain(self):
        private = str(self.prefix / "go/bin/go")
        with patch.object(env.shutil, "which", return_value="/old/go"), \
             patch.object(env, "probe_go", side_effect=lambda path, minimum: "1.27.1" if path == private else None):
            self.assertEqual(env.find_go("1.26.2", self.prefix), (private, "1.27.1"))

    def test_reusing_newer_go_has_no_downloads_or_writes(self):
        with patch.object(env, "find_go", return_value=("/existing/go", "1.27.1")), \
             patch.object(env, "go_asset") as asset:
            self.assertEqual(env.ensure_go("1.26.2", self.prefix), ("/existing/go", "1.27.1"))
        asset.assert_not_called()
        self.assertFalse(self.prefix.exists())

    def test_check_never_downloads_or_creates_directories(self):
        with patch.object(env, "find_go", return_value=None), patch.object(env, "go_asset") as asset:
            with self.assertRaisesRegex(RuntimeError, "setup-cloud-env"):
                env.ensure_go("1.26.2", self.prefix, check=True)
        asset.assert_not_called()
        self.assertFalse(self.prefix.exists())

    def test_metadata_requires_official_linux_amd64_asset(self):
        asset = {"os": "linux", "arch": "amd64", "kind": "archive", "filename": "go1.26.2.linux-amd64.tar.gz", "sha256": "a" * 64}
        payload = [{"version": "go1.26.2", "files": [asset]}]
        with patch.object(env.urllib.request, "urlopen", return_value=io.BytesIO(json.dumps(payload).encode())):
            self.assertEqual(env.go_asset("1.26.2"), asset)
        asset["filename"] = "../../other.tar.gz"
        with patch.object(env.urllib.request, "urlopen", return_value=io.BytesIO(json.dumps(payload).encode())):
            with self.assertRaisesRegex(RuntimeError, "元数据"):
                env.go_asset("1.26.2")

    def test_checksum_mismatch_never_extracts_or_publishes_go(self):
        asset = {"filename": "go1.26.2.linux-amd64.tar.gz", "sha256": "0" * 64}
        with patch.object(env, "find_go", return_value=None), patch.object(env, "go_asset", return_value=asset), \
             patch.object(env.urllib.request, "urlopen", return_value=io.BytesIO(b"corrupt")), \
             patch.object(env, "extract_go") as extract, contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaisesRegex(RuntimeError, "SHA256"):
                env.ensure_go("1.26.2", self.prefix)
        extract.assert_not_called()
        self.assertEqual(list(self.prefix.iterdir()), [])

    def archive(self, name="go/bin/go", kind=tarfile.REGTYPE):
        path = self.root / "fixture.tar.gz"
        with tarfile.open(path, "w:gz") as archive:
            member = tarfile.TarInfo(name)
            member.type = kind
            member.mode = 0o755
            if kind == tarfile.REGTYPE:
                data = b"fake Go"
                member.size = len(data)
                archive.addfile(member, io.BytesIO(data))
            else:
                member.linkname = "../../outside"
                archive.addfile(member)
        return path

    def test_extraction_rejects_traversal_links_and_unexpected_roots(self):
        for name, kind in (("go/../../outside", tarfile.REGTYPE), ("/outside", tarfile.REGTYPE),
                           ("other/bin/go", tarfile.REGTYPE), ("go/bin/go", tarfile.SYMTYPE)):
            with self.subTest(name=name, kind=kind), self.assertRaisesRegex(RuntimeError, "不安全"):
                env.extract_go(self.archive(name, kind), self.root / "stage")

    def test_verified_private_install_and_repeat_run(self):
        archive = self.archive()
        data = archive.read_bytes()
        asset = {"filename": "go1.26.2.linux-amd64.tar.gz", "sha256": hashlib.sha256(data).hexdigest()}
        with patch.object(env, "find_go", return_value=None), patch.object(env, "go_asset", return_value=asset), \
             patch.object(env.urllib.request, "urlopen", return_value=io.BytesIO(data)), \
             patch.object(env, "probe_go", return_value="1.26.2"), contextlib.redirect_stdout(io.StringIO()):
            path, version = env.ensure_go("1.26.2", self.prefix)
        self.assertEqual(version, "1.26.2")
        self.assertEqual(Path(path).read_bytes(), b"fake Go")
        self.assertTrue(os.access(path, os.X_OK))
        self.assertEqual((self.prefix / "go").readlink(), Path("go-1.26.2"))
        with patch.object(env, "find_go", return_value=(path, version)), patch.object(env, "go_asset") as fetch:
            self.assertEqual(env.ensure_go("1.26.2", self.prefix), (path, version))
        fetch.assert_not_called()
        self.assertFalse(list(self.prefix.glob(".prepare-*")))

    def test_existing_invalid_directory_is_preserved(self):
        target = self.prefix / "go-1.26.2"
        target.mkdir(parents=True)
        marker = target / "keep.txt"
        marker.write_text("keep")
        with patch.object(env, "find_go", return_value=None), patch.object(env, "probe_go", return_value=None), \
             patch.object(env, "go_asset") as fetch:
            with self.assertRaisesRegex(RuntimeError, "已有无效工具目录"):
                env.ensure_go("1.26.2", self.prefix)
        self.assertEqual(marker.read_text(), "keep")
        fetch.assert_not_called()


class UbuntuBootstrapTests(unittest.TestCase):
    def run_bootstrap(self, args=(), missing=(), apt_ok=True, python_ok=True, version="26.04"):
        with tempfile.TemporaryDirectory(prefix="yandu bootstrap ") as temporary:
            stage = Path(temporary)
            commands = stage / "commands"
            commands.mkdir()
            bash = shutil.which("bash")
            release = stage / "os-release"
            release.write_text(f"ID=ubuntu\nVERSION_ID={shlex.quote(version)}\n")
            script = Path(__file__).with_name("setup-cloud-env.sh").read_text()
            script = script.replace(". /etc/os-release", ". " + shlex.quote(str(release))).replace("$EUID", "0")
            script = script.replace("/usr/bin/python3", str(stage / "missing-system-python"))
            entry = stage / "setup-cloud-env.sh"
            entry.write_text(script)
            for name in ("dirname",):
                (commands / name).symlink_to(shutil.which(name))
            fake = {
                "uname": "case $1 in -s) echo Linux;; -m) echo x86_64;; esac",
                "systemctl": "exit 99", "useradd": "exit 99",
                "dpkg-query": 'case $3 in ' + "|".join(missing or ("no-missing-package",)) + ") exit 1;; *) printf 'install ok installed';; esac",
                "apt-get": 'printf "apt:<%s>\\n" "$*"; exit ' + ("0" if apt_ok else "42"),
                "python3": 'if [[ $1 == -c ]]; then exit ' + ("0" if python_ok else "1") + '; fi; printf "python:<%s>\\n" "$*"',
            }
            for name, body in fake.items():
                path = commands / name
                path.write_text(f"#!{bash}\n{body}\n")
                path.chmod(0o755)
            return subprocess.run([bash, str(entry), *args], env={**os.environ, "PATH": str(commands), "BASH_ENV": "/dev/null"},
                                  capture_output=True, text=True, timeout=10)

    def test_missing_packages_are_installed_before_go_resolution(self):
        result = self.run_bootstrap(missing=("python3", "acl"))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("apt:<update>", result.stdout)
        self.assertIn("apt:<install -y --no-install-recommends python3 acl>", result.stdout)
        self.assertLess(result.stdout.index("apt:<install"), result.stdout.index("python:<"))

    def test_repeated_run_skips_apt_when_packages_exist(self):
        result = self.run_bootstrap()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("apt:<", result.stdout)
        self.assertIn("cloud_env.py", result.stdout)

    def test_check_and_dry_run_never_install_packages(self):
        for flag, code in (("--check", 1), ("--dry-run", 0)):
            with self.subTest(flag=flag):
                result = self.run_bootstrap((flag,), missing=("python3", "acl"), python_ok=False)
                self.assertEqual(result.returncode, code, result.stderr)
                self.assertNotIn("apt:<", result.stdout)
                self.assertNotIn("python:<", result.stdout)

    def test_check_passes_read_only_flag_to_runtime_resolver(self):
        result = self.run_bootstrap(("--check",))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("cloud_env.py --check", result.stdout)
        self.assertNotIn("apt:<", result.stdout)

    def test_failed_apt_stops_before_runtime_preparation(self):
        result = self.run_bootstrap(missing=("acl",), apt_ok=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("python:<", result.stdout)

    def test_old_ubuntu_is_rejected_before_package_commands(self):
        result = self.run_bootstrap(version="22.04")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Ubuntu 24.04", result.stderr)
        self.assertNotIn("apt:<", result.stdout)


if __name__ == "__main__":
    unittest.main()
