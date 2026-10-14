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
                      architecture="x86_64", missing=None):
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
            installer = (SCRIPTS / "install-cloud-storage.sh").read_text()
            self.assertIn(". /etc/os-release", installer)
            installer = installer.replace(". /etc/os-release", ". " + shlex.quote(str(release)))
            installer = installer.replace("$EUID", "0")
            (stage / "install-cloud-storage.sh").write_text(installer)
            shutil.copyfile(SCRIPTS / "install-cloud.sh", stage / "install-cloud.sh")
            for name in ("bash", "dirname"):
                (commands / name).symlink_to(shutil.which(name))
            uname = commands / "uname"
            uname.write_text(f"#!{bash}\ncase $1 in -s) echo {shlex.quote(system)};; -m) echo {shlex.quote(architecture)};; *) exit 99;; esac\n")
            uname.chmod(0o755)
            for name in ("systemctl", "useradd", "install", "chown", "chmod", "id", "ln", "apt-get"):
                if name == missing:
                    continue
                command = commands / name
                command.write_text(f"#!{bash}\necho 'dry-run unexpectedly invoked {name}' >&2\nexit 99\n")
                command.chmod(0o755)
            return subprocess.run(
                [bash, str(stage / "install-cloud.sh"), "--mode", "storage-only",
                 "--server", "203.0.113.10", "--device", "owner", "--dry-run"],
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


if __name__ == "__main__":
    unittest.main()
