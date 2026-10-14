"""Exercise storage-only dry-run admission without touching a host installation."""
from pathlib import Path
import os
import shlex
import shutil
import subprocess
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parent


class CloudInstallPreflightTests(unittest.TestCase):
    def run_installer(self, version="26.04", distribution="ubuntu", system="Linux",
                      architecture="x86_64", missing=None, source=False,
                      dry_run=True, python_available=True, existing=False, environment_ok=True):
        with tempfile.TemporaryDirectory(prefix="yandu cloud preflight ") as temporary:
            stage = Path(temporary)
            commands = stage / "commands"
            commands.mkdir()
            bash = shutil.which("bash")
            self.assertIsNotNone(bash)
            # Substitute only the host OS facts and effective UID in a temporary
            # copy. All argument parsing, admission checks and dry-run logic run
            # from the real installer; no privileged installation is performed.
            release = stage / "os-release"
            release.write_text(f"ID={shlex.quote(distribution)}\nVERSION_ID={shlex.quote(version)}\n")
            scripts = stage / "scripts" if source else stage
            scripts.mkdir(exist_ok=True)
            installer = (SCRIPTS / "install-cloud-storage.sh").read_text()
            self.assertIn(". /etc/os-release", installer)
            installer = installer.replace(". /etc/os-release", ". " + shlex.quote(str(release)))
            installer = installer.replace("$EUID", "0")
            installer = installer.replace("/usr/bin/python3", str(stage / "missing-system-python"))
            # Isolate ownership checks as well as OS facts. Non-dry-run cases
            # stop at the source handoff and never run an installer on the host.
            managed = stage / "etc/yandu"
            if existing:
                managed.mkdir(parents=True)
            installer = installer.replace("[[ ! -e /etc/yandu ]]", f"[[ ! -e {shlex.quote(str(managed))} ]]")
            installer = installer.replace("[[ -e /usr/local/bin/yandu || -L /usr/local/bin/yandu ]]", "[[ 0 == 1 ]]")
            (scripts / "install-cloud-storage.sh").write_text(installer)
            shutil.copyfile(SCRIPTS / "install-cloud.sh", scripts / "install-cloud.sh")
            if source:
                shutil.copyfile(SCRIPTS.parent / "install-cloud.sh", stage / "install-cloud.sh")
                (stage / "go.mod").write_text("module fixture\ngo 1.26.2\n")
                (stage / "cmd/yandu-cloud").mkdir(parents=True)
                shutil.copyfile(SCRIPTS / "prepare-cloud-install.py", scripts / "prepare-cloud-install.py")
                (scripts / "setup-cloud-env.sh").write_text(
                    "echo 'environment preparation'\nexit " + ("0" if environment_ok else "1") + "\n")
            for name in ("bash", "dirname"):
                (commands / name).symlink_to(shutil.which(name))
            uname = commands / "uname"
            uname.write_text(f"#!{bash}\ncase $1 in -s) echo {shlex.quote(system)};; -m) echo {shlex.quote(architecture)};; *) exit 99;; esac\n")
            uname.chmod(0o755)
            for name in ("systemctl", "useradd", "install", "chown", "chmod", "id", "ln", "apt-get"):
                if name == missing:
                    continue
                command = commands / name
                command.write_text(f"#!{bash}\nexit 1\n" if name == "id" else f"#!{bash}\necho 'dry-run unexpectedly invoked {name}' >&2\nexit 99\n")
                command.chmod(0o755)
            if python_available:
                python = commands / "python3"
                python.write_text(f"#!{bash}\nprintf 'source preparation:'\nprintf '<%s>' \"$@\"\nprintf '\\n'\n")
                python.chmod(0o755)
            arguments = [bash, str(stage / "install-cloud.sh"), "--mode", "storage-only",
                         "--server", "203.0.113.10", "--device", "owner"]
            if dry_run:
                arguments.append("--dry-run")
            return subprocess.run(
                arguments,
                env={**os.environ, "PATH": str(commands), "BASH_ENV": "/dev/null"},
                text=True, capture_output=True, timeout=10,
            )

    def test_supported_and_newer_ubuntu_versions_pass_without_installing(self):
        for version in ("24.04", "24.10", "26.04", "26.10"):
            with self.subTest(version=version):
                result = self.run_installer(version=version)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("storage-only", result.stdout)
                self.assertNotIn("unexpectedly invoked", result.stderr)

    def test_old_malformed_and_non_ubuntu_releases_explain_rejection(self):
        for distribution, version in (("ubuntu", "22.04"), ("ubuntu", "24.03"),
                                      ("ubuntu", ""), ("ubuntu", "invalid"),
                                      ("debian", "26.04")):
            with self.subTest(distribution=distribution, version=version):
                result = self.run_installer(version=version, distribution=distribution)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("不支持的系统", result.stderr)

    def test_platform_and_architecture_are_still_required(self):
        for system, architecture in (("Darwin", "x86_64"), ("Linux", "aarch64")):
            with self.subTest(system=system, architecture=architecture):
                result = self.run_installer(system=system, architecture=architecture)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("需要 Ubuntu x64", result.stdout + result.stderr)

    def test_missing_dependency_is_reported_before_installation(self):
        for name in ("systemctl", "useradd", "apt-get"):
            with self.subTest(name=name):
                result = self.run_installer(missing=name)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("缺少云端安装所需命令", result.stderr)
                self.assertIn(name, result.stderr)
                self.assertNotIn("unexpectedly invoked", result.stderr)

    def test_repository_root_entry_hands_off_source_preparation(self):
        result = self.run_installer(source=True, dry_run=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("source preparation:", result.stdout)
        self.assertLess(result.stdout.index("environment preparation"), result.stdout.index("source preparation:"))
        self.assertIn("prepare-cloud-install.py", result.stdout)
        self.assertIn("<--server><203.0.113.10><--device><owner>", result.stdout)
        self.assertNotIn("unexpectedly invoked", result.stderr)

    def test_source_dry_run_never_compiles_or_downloads(self):
        result = self.run_installer(source=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("storage-only", result.stdout)
        self.assertNotIn("source preparation:", result.stdout)
        self.assertNotIn("environment preparation", result.stdout)

    def test_failed_environment_never_builds_or_installs(self):
        result = self.run_installer(source=True, dry_run=False, environment_ok=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("environment preparation", result.stdout)
        self.assertNotIn("source preparation:", result.stdout)

    def test_missing_python_explains_source_prerequisite(self):
        result = self.run_installer(source=True, dry_run=False, python_available=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Python 3.10+", result.stderr)

    def test_existing_installation_rejects_before_source_preparation(self):
        result = self.run_installer(source=True, dry_run=False, existing=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("已有安装", result.stderr)
        self.assertNotIn("source preparation:", result.stdout)
        self.assertNotIn("environment preparation", result.stdout)


if __name__ == "__main__":
    unittest.main()
