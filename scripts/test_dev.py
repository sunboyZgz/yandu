"""Source launcher tests, including Windows command construction and process ownership."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import signal
import socket
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch, Mock

spec = importlib.util.spec_from_file_location("yandu_dev", Path(__file__).with_name("dev.py"))
dev = importlib.util.module_from_spec(spec)
spec.loader.exec_module(dev)


class LauncherTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="yandu source & spaces ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()

    def test_existing_matching_npm_dependencies_are_reused(self):
        directory=self.root/'web';(directory/'node_modules').mkdir(parents=True)
        packages={'node_modules/typescript':{'version':'5.9.3','integrity':'sha512-fixture'}}
        (directory/'package-lock.json').write_text(json.dumps({'packages':{'':{'name':'fixture'},**packages}}))
        (directory/'node_modules/.package-lock.json').write_text(json.dumps({'packages':packages}))
        node=self.root/'node'
        with patch.object(dev,'execute') as execute,patch.object(dev.subprocess,'run',return_value=subprocess.CompletedProcess([],0)):
            dev.ensure_npm(self.root,'web',node)
        execute.assert_not_called()
        self.assertTrue((self.root/'.local/npm-stamps/web.sha256').is_file())

    def test_changed_npm_lock_installs_only_project_dependencies(self):
        directory=self.root/'media';(directory/'node_modules').mkdir(parents=True)
        (directory/'package-lock.json').write_text(json.dumps({'packages':{'node_modules/sharp':{'version':'0.35.5','integrity':'new'}}}))
        (directory/'node_modules/.package-lock.json').write_text(json.dumps({'packages':{'node_modules/sharp':{'version':'old','integrity':'old'}}}))
        node=self.root/'Node 中文/bin/node';entry=node.parent.parent/'lib/node_modules/npm/bin/npm-cli.js';entry.parent.mkdir(parents=True);entry.write_text('fixture')
        with patch.object(dev,'execute') as execute:
            dev.ensure_npm(self.root,'media',node)
        self.assertEqual(execute.call_count,1)
        args=execute.call_args.args[0]
        self.assertEqual(args,[str(node),str(entry),'--prefix',str(directory),'ci'])
        self.assertNotIn('-g',args)

    def test_native_tool_builders_receive_the_selected_python_interpreter(self):
        node = self.root / 'Node path/node.exe'
        entry = node.parent / 'node_modules/npm/bin/npm-cli.js'
        entry.parent.mkdir(parents=True)
        entry.write_text('fixture')
        prepared = []
        def execute(args, root, **kwargs):
            if args[0] == 'go' and '-o' in args:
                Path(args[args.index('-o') + 1]).write_bytes(b'compiled fixture')
            if 'tools' in args and 'prepare' in args:
                prepared.append(kwargs['env']['YANDU_BUILD_PYTHON'])
                return subprocess.CompletedProcess(args, 0, json.dumps({'tools': {'node': {'path': str(node)}}}).encode())
            return subprocess.CompletedProcess(args, 0)
        with patch.object(dev, 'host_target', return_value='windows-amd64'), \
             patch.object(dev, 'check_output_files'), patch.object(dev, 'tool', side_effect=lambda name: name), \
             patch.object(dev, 'execute', side_effect=execute), patch.object(dev, 'ensure_npm'), \
             contextlib.redirect_stdout(io.StringIO()):
            dev.build(self.root, self.root / 'bin', 'windows-amd64', role='local', no_media=True)
        self.assertEqual(prepared, [sys.executable, sys.executable])

    def test_windows_plan_has_exe_and_only_windows_components(self):
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            result = dev.main(["plan", "--target", "windows-amd64", "--state-dir", str(self.root / "state & 中文")])
        plan = json.loads(output.getvalue())
        self.assertEqual(result, 0)
        self.assertEqual(plan["target"], "windows-amd64")
        self.assertTrue(plan["binary"].endswith("yandu.exe"))
        self.assertEqual(len(plan["assets"]), 2)
        self.assertTrue(all(name.endswith("windows_amd64.zip") for name in plan["assets"]))
        self.assertEqual(plan["agentArgs"][1:], ["--state-dir", str(self.root / "state & 中文"), "serve"])

    def test_windows_npm_uses_node_without_cmd_shell(self):
        directory = self.root / "Node & Program Files"
        directory.mkdir()
        npm = directory / "npm.cmd"
        node = directory / "node.exe"
        entry = directory / "node_modules/npm/bin/npm-cli.js"
        entry.parent.mkdir(parents=True)
        for path in (npm, node, entry):
            path.write_text("fixture")
        with patch.object(dev.sys, "platform", "win32"), patch.object(dev, "tool", side_effect=lambda name: str(npm if name == "npm" else node)):
            command = dev.npm_command("--prefix", str(self.root), "ci")
        self.assertEqual(command, [str(node), str(entry), "--prefix", str(self.root), "ci"])
        with patch.object(dev.subprocess, "run") as run:
            dev.execute(command, self.root)
        run.assert_called_once_with(command, cwd=self.root, check=True)

    def test_busy_port_rejects_before_build_or_process_creation(self):
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            listener.listen()
            port = listener.getsockname()[1]
            with patch.object(dev, "PORTS", (port,)), patch.object(dev, "require_native"), patch.object(dev, "build") as build, patch.object(dev.subprocess, "Popen") as popen:
                with self.assertRaisesRegex(dev.DevError, "in use"):
                    dev.run_agent(self.root, self.root / "bin", self.root / "state", "windows-amd64", False, 1)
            build.assert_not_called()
            popen.assert_not_called()

    def test_cross_target_run_is_rejected_before_mutation(self):
        with patch.object(dev, "host_target", return_value="darwin-arm64"), patch.object(dev, "build") as build:
            with self.assertRaisesRegex(dev.DevError, "Cannot run"):
                dev.run_agent(self.root, self.root / "bin", self.root / "state", "windows-amd64", False, 1)
        build.assert_not_called()

    def test_unready_agent_is_stopped_and_browser_not_opened(self):
        child = Mock()
        child.poll.return_value = None
        with patch.object(dev, "require_native"), patch.object(dev, "ensure_ports_free"), patch.object(dev, "build"), patch.object(dev, "prepare_state"), patch.object(dev.subprocess, "Popen", return_value=child), patch.object(dev, "read_status", return_value={"components": {"agent": True, "fileService": False}}), patch.object(dev.time, "monotonic", side_effect=[0, 0, 2]), patch.object(dev.time, "sleep"), patch.object(dev, "stop_child") as stop, patch.object(dev, "execute") as execute:
            with self.assertRaisesRegex(dev.DevError, "not ready"):
                dev.run_agent(self.root, self.root / "bin", self.root / "state", "windows-amd64", False, 1)
        stop.assert_called_once_with(child)
        execute.assert_not_called()

    def test_verified_agent_opens_ui_and_interrupt_stops_only_owned_child(self):
        child = Mock()
        child.poll.return_value = None
        child.wait.side_effect = KeyboardInterrupt
        bindir = self.root / "bin & 中文"
        state = self.root / "state & 中文"
        with patch.object(dev, "require_native"), patch.object(dev, "ensure_ports_free"), patch.object(dev, "build"), patch.object(dev, "prepare_state"), patch.object(dev.subprocess, "Popen", return_value=child) as popen, patch.object(dev, "read_status", return_value={"components": {"agent": True, "fileService": True}}), patch.object(dev, "stop_child") as stop, patch.object(dev, "execute") as execute, contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaises(KeyboardInterrupt):
                dev.run_agent(self.root, bindir, state, "windows-amd64", False, 1)
        self.assertEqual(popen.call_args.args[0], [str(bindir / "yandu.exe"), "--state-dir", str(state), "serve"])
        execute.assert_called_once_with([str(bindir / "yandu.exe"), "--state-dir", str(state), "ui", "--require-running"], self.root)
        stop.assert_called_once_with(child)

    def test_windows_stop_uses_console_break_and_does_not_force_kill(self):
        child = Mock()
        child.poll.return_value = None
        with patch.object(dev.sys, "platform", "win32"):
            dev.stop_child(child)
        child.send_signal.assert_called_once_with(getattr(signal, "CTRL_BREAK_EVENT", 1))
        child.wait.assert_called_once_with(timeout=15)
        child.terminate.assert_not_called()

    def test_finished_child_is_not_signalled(self):
        child = Mock()
        child.poll.return_value = 0
        dev.stop_child(child)
        child.send_signal.assert_not_called()

    def test_ui_refuses_unverified_instance_without_launching_it(self):
        (self.root / "bin").mkdir()
        (self.root / "bin/yandu.exe").write_bytes(b"fixture")
        with patch.object(dev, "ROOT", self.root), patch.object(dev, "require_native"), patch.object(dev, "read_status", side_effect=subprocess.CalledProcessError(1, "status")), patch.object(dev, "execute") as execute, contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(dev.main(["ui", "--target", "windows-amd64", "--bin-dir", "bin", "--state-dir", "state"]), 1)
        execute.assert_not_called()

    def test_windows_acl_handles_aad_sid_and_preserves_argument_boundaries(self):
        state = self.root / "private state & 中文"
        sid = b"S-1-12-1-1234567890-1111111111-2222222222-3333333333"
        who = subprocess.CompletedProcess([], 0, b'"AAD\\user","' + sid + b'"\r\n')
        with patch.object(dev.sys, "platform", "win32"), patch.object(dev, "tool", side_effect=lambda name: name), patch.object(dev, "execute", side_effect=[who, subprocess.CompletedProcess([], 0)]) as execute:
            dev.prepare_state(self.root, state)
        acl = execute.call_args_list[1].args[0]
        self.assertEqual(acl[:4], ["icacls", str(state), "/inheritance:r", "/grant:r"])
        self.assertIn("*" + sid.decode() + ":(OI)(CI)F", acl)
        self.assertFalse(any("S-1-5-19:" in part for part in acl))

    def test_windows_acl_uses_csv_sid_column_for_unicode_output(self):
        state = self.root / "private state"
        sid = "S-1-5-21-11-22-33-1001"
        raw = ('"DOMAIN\\用户S-1-5-18","' + sid + '"\r\n').encode('utf-16')
        with patch.object(dev.sys, "platform", "win32"), patch.object(dev, "tool", side_effect=lambda name: name), patch.object(dev, "execute", side_effect=[subprocess.CompletedProcess([], 0, raw), subprocess.CompletedProcess([], 0)]) as execute:
            dev.prepare_state(self.root, state)
        self.assertEqual(execute.call_args_list[1].args[0][-1], '*' + sid + ':(OI)(CI)F')


if __name__ == "__main__":
    unittest.main()
