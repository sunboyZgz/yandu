"""Verify source upgrade scope, failure boundaries, and the real upgrade handoff."""
import contextlib
import importlib.util
import io
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from test_cloud_upgrade import fixture


SCRIPTS = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("prepare_cloud_upgrade", SCRIPTS / "prepare-cloud-upgrade.py")
source = importlib.util.module_from_spec(spec)
spec.loader.exec_module(source)


class SourceUpgradeTests(unittest.TestCase):
    def test_cli_and_gateway_build_without_preparing_frps(self):
        for component in ("cli", "gateway"):
            with self.subTest(component=component), fixture() as f:
                checkout = f.root / "source checkout"
                (checkout / "scripts").mkdir(parents=True)
                shutil.copy2(f.script, checkout / "scripts/upgrade-cloud.sh")
                stages = []

                def build(root, stage, go):
                    stages.append(stage)
                    shutil.copy2(f.package / "yandu-cloud", stage / "yandu-cloud")

                with patch.object(source, "ROOT", checkout), patch.object(source, "preflight", return_value="/tools/go"), \
                     patch.object(source, "build_cloud", side_effect=build), patch.object(source, "prepare_frps") as frps, \
                     patch.dict(os.environ, f.env), contextlib.redirect_stdout(io.StringIO()):
                    self.assertEqual(source.main(["--component", component, "--drain-timeout", "600"]), 0)
                frps.assert_not_called()
                self.assertFalse(stages[0].exists())
                self.assertEqual(f.restarts(), [] if component == "cli" else ["yandu-gateway"])
                target = "yandu-cloud" if component == "cli" else "yandu-gateway"
                self.assertEqual((f.lib / target).read_bytes(), f.new)
                f.assert_protected(self)

    def test_tunnel_handoff_preserves_disconnect_gate_and_allows_explicit_restart(self):
        for allow in (False, True):
            with self.subTest(allow=allow), fixture() as f:
                checkout = f.root / "source checkout"
                (checkout / "scripts").mkdir(parents=True)
                shutil.copy2(f.script, checkout / "scripts/upgrade-cloud.sh")
                old_frps = (f.lib / "frps").read_bytes()
                stages = []

                def build(root, stage, go):
                    stages.append(stage)
                    shutil.copy2(f.package / "yandu-cloud", stage / "yandu-cloud")

                def prepare(root, stage):
                    (stage / "bin").mkdir()
                    shutil.copy2(f.package / "bin/frps", stage / "bin/frps")

                with patch.object(source, "ROOT", checkout), patch.object(source, "preflight", return_value="/tools/go"), \
                     patch.object(source, "build_cloud", side_effect=build), patch.object(source, "prepare_frps", side_effect=prepare) as frps, \
                     patch.dict(os.environ, f.env), contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                    args = ["--component", "tunnel"] + (["--allow-disconnect"] if allow else [])
                    self.assertEqual(source.main(args), 0 if allow else 1)
                frps.assert_called_once()
                self.assertFalse(stages[0].exists())
                self.assertEqual(f.restarts(), ["yandu-frps"] if allow else [])
                self.assertEqual((f.lib / "yandu-cloud").read_bytes(), f.old)
                self.assertEqual((f.lib / "frps").read_bytes(), (f.package / "bin/frps").read_bytes() if allow else old_frps)
                f.assert_protected(self)

    def test_all_prepares_tunnel_and_forwards_all_flags(self):
        stages = []
        def build(root, stage, go):
            stages.append(stage)
        with patch.object(source, "preflight", return_value="/tools/go"), \
             patch.object(source, "build_cloud", side_effect=build), patch.object(source, "prepare_frps") as frps, \
             patch.object(source.shutil, "copy2"), patch.object(source.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as run, \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(source.main(["--component", "all", "--drain-timeout", "00900", "--allow-disconnect"]), 0)
        frps.assert_called_once_with(source.ROOT, stages[0])
        self.assertEqual(run.call_args.args[0], ["bash", str(stages[0] / "upgrade-cloud.sh"), "--component", "all",
                                                "--drain-timeout", "900", "--allow-disconnect"])
        self.assertEqual(run.call_args.kwargs, {})  # Keep terminal streams attached.
        self.assertFalse(stages[0].exists())

    def test_preview_never_builds_downloads_stages_or_invokes_upgrader(self):
        output = io.StringIO()
        with patch.object(source, "preflight", return_value="/tools/go"), \
             patch.object(source, "build_cloud") as build, patch.object(source, "prepare_frps") as frps, \
             patch.object(source.tempfile, "TemporaryDirectory") as temporary, patch.object(source.subprocess, "run") as run, \
             contextlib.redirect_stdout(output):
            self.assertEqual(source.main(["--component", "all", "--dry-run"]), 0)
        for operation in (build, frps, temporary, run):
            operation.assert_not_called()
        self.assertIn("是否需要重启须在正式构建后", output.getvalue())

    def test_preparation_failure_never_invokes_transaction_and_cleans_stage(self):
        for step in ("build", "frps"):
            stages = []
            def build(root, stage, go):
                stages.append(stage)
                if step == "build":
                    raise subprocess.CalledProcessError(1, "go build")
            with self.subTest(step=step), patch.object(source, "preflight", return_value="/tools/go"), \
                 patch.object(source, "build_cloud", side_effect=build), \
                 patch.object(source, "prepare_frps", side_effect=RuntimeError("frps failed")), \
                 patch.object(source.subprocess, "run") as run, contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(source.main(["--component", "all"]), 1)
            run.assert_not_called()
            self.assertFalse(stages[0].exists())

    def test_failed_environment_or_invalid_flags_never_build(self):
        with patch.object(source, "preflight", side_effect=RuntimeError("not installed")), \
             patch.object(source, "build_cloud") as build, contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(source.main([]), 1)
            for args in (["--component", "unknown"], ["--drain-timeout", "0"], ["--drain-timeout", "86401"]):
                with self.assertRaises(SystemExit):
                    source.main(args)
        build.assert_not_called()

    def test_preflight_checks_host_installation_systemd_and_existing_go(self):
        with fixture() as f, patch.object(source, "CONFIG", f.config), patch.object(source, "LIB", f.lib), \
             patch.object(source, "UNITS", f.units), patch.object(source, "require_host", return_value="linux-amd64"), \
             patch.object(source.platform, "freedesktop_os_release", return_value={"ID": "ubuntu", "VERSION_ID": "26.04"}), \
             patch.object(source.os, "geteuid", return_value=0, create=True), patch.dict(os.environ, f.env), \
             patch.object(source, "find_go", return_value=("/private/go", "1.27.1")) as find:
            self.assertEqual(source.preflight("cli"), "/private/go")
            find.assert_called_once_with("1.26.2")
            with patch.object(source, "find_go", return_value=None), self.assertRaisesRegex(RuntimeError, "allow-system-install"):
                source.preflight("cli")
            with patch.object(source.os, "geteuid", return_value=1000), self.assertRaisesRegex(RuntimeError, "sudo"):
                source.preflight("cli")
            with patch.object(source, "require_host", return_value="darwin-arm64"), self.assertRaisesRegex(RuntimeError, "Ubuntu"):
                source.preflight("cli")
            with patch.object(source.platform, "freedesktop_os_release", return_value={"ID": "debian", "VERSION_ID": "13"}), self.assertRaisesRegex(RuntimeError, "Ubuntu"):
                source.preflight("cli")
            (f.config / "storage-owner.json").unlink()
            with self.assertRaisesRegex(RuntimeError, "首次部署"):
                source.preflight("cli")

    def test_root_and_scripts_entry_resolve_checkout_from_another_directory(self):
        with tempfile.TemporaryDirectory(prefix="yandu source entry & spaces ") as temporary:
            root = Path(temporary)
            (root / "scripts").mkdir()
            (root / "cmd/yandu-cloud").mkdir(parents=True)
            (root / "go.mod").write_text("module fixture\n")
            for path in ("upgrade-cloud.sh", "scripts/upgrade-cloud.sh"):
                shutil.copy2(SCRIPTS.parent / path, root / path)
            (root / "scripts/prepare-cloud-upgrade.py").write_text("import json,sys\nprint(json.dumps(sys.argv[1:]))\n")
            args = ["--component", "gateway", "--drain-timeout", "600", "--dry-run"]
            for entry in (root / "upgrade-cloud.sh", root / "scripts/upgrade-cloud.sh"):
                result = subprocess.run(["bash", str(entry), *args], cwd="/", capture_output=True, text=True, check=True)
                self.assertIn('["--component", "gateway", "--drain-timeout", "600", "--dry-run"]', result.stdout)


if __name__ == "__main__":
    unittest.main()
