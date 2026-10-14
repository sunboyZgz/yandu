"""Test source cloud preparation and terminal handoff without root or downloads."""
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location("prepare_cloud", Path(__file__).with_name("prepare-cloud-install.py"))
cloud = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cloud)


class CloudSourcePreparationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="yandu cloud source & spaces ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.stage = self.root / "stage"
        self.stage.mkdir()
        (self.root / "scripts").mkdir()
        for name in ("install-cloud.sh", "install-cloud-storage.sh"):
            (self.root / "scripts" / name).write_text("installer fixture: " + name)
        self.frps = self.root / "existing frps"
        self.frps.write_bytes(b"verified existing frps")
        self.frps.chmod(0o755)

    def tool_config(self):
        return json.dumps({"tools": {"frps": {"path": str(self.frps), "sha256": hashlib.sha256(self.frps.read_bytes()).hexdigest()}}})

    def test_cloud_only_build_reuses_registered_frps_and_stages_installers(self):
        with patch.object(cloud, "go_tool", return_value="/tools/go"), \
             patch.object(cloud.subprocess, "run") as run, \
             patch.object(cloud.subprocess, "check_output", return_value=self.tool_config()) as prepare, \
             contextlib.redirect_stdout(io.StringIO()):
            cloud.prepare(self.root, self.stage)
        run.assert_called_once()
        command = run.call_args.args[0]
        self.assertEqual(command[0], "/tools/go")
        self.assertEqual(command[-1], "./cmd/yandu-cloud")
        self.assertEqual(run.call_args.kwargs["cwd"], self.root)
        self.assertEqual({name: run.call_args.kwargs["env"][name] for name in ("GOOS", "GOARCH", "CGO_ENABLED")},
                         {"GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0"})
        self.assertIn("cloud", prepare.call_args.args[0])
        self.assertIn("--json", prepare.call_args.args[0])
        self.assertEqual((self.stage / "bin/frps").read_bytes(), self.frps.read_bytes())
        for name in ("install-cloud.sh", "install-cloud-storage.sh"):
            self.assertEqual((self.stage / name).read_bytes(), (self.root / "scripts" / name).read_bytes())

    def test_changed_frps_stops_before_installer_is_staged(self):
        config = self.tool_config()
        self.frps.write_bytes(b"changed after verification")
        with patch.object(cloud, "go_tool", return_value="/tools/go"), \
             patch.object(cloud.subprocess, "run"), \
             patch.object(cloud.subprocess, "check_output", return_value=config), \
             contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaisesRegex(RuntimeError, "frps"):
                cloud.prepare(self.root, self.stage)
        self.assertFalse((self.stage / "install-cloud.sh").exists())

    def test_failed_build_never_prepares_tools(self):
        with patch.object(cloud, "go_tool", return_value="/tools/go"), \
             patch.object(cloud.subprocess, "run", side_effect=subprocess.CalledProcessError(1, "go build")), \
             patch.object(cloud.subprocess, "check_output") as prepare, \
             contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaises(subprocess.CalledProcessError):
                cloud.prepare(self.root, self.stage)
        prepare.assert_not_called()

    def test_main_preserves_arguments_and_terminal_then_cleans_staging(self):
        with patch.object(cloud, "prepare") as prepare, \
             patch.object(cloud.subprocess, "run") as run, \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(cloud.main(["--server", "203.0.113.10", "--device", "owner"]), 0)
        stage = prepare.call_args.args[1]
        self.assertEqual(run.call_args.args[0], ["bash", str(stage / "install-cloud.sh"), "--mode", "storage-only",
                                               "--server", "203.0.113.10", "--device", "owner"])
        self.assertEqual(run.call_args.kwargs, {"check": True})
        self.assertFalse(stage.exists())

    def test_main_cleans_staging_after_preparation_fails(self):
        with patch.object(cloud, "prepare", side_effect=RuntimeError("build failed")) as prepare, \
             patch.object(cloud.subprocess, "run") as run, \
             contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(cloud.main(["--server", "203.0.113.10"]), 1)
        run.assert_not_called()
        self.assertFalse(prepare.call_args.args[1].exists())

    def test_missing_go_explains_prerequisite(self):
        with patch.object(cloud.shutil, "which", return_value=None), patch.object(Path, "is_file", return_value=False):
            with self.assertRaisesRegex(RuntimeError, r"Go 1\.26\.2\+"):
                cloud.go_tool()


if __name__ == "__main__":
    unittest.main()
