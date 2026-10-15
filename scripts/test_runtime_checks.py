"""Runtime admission is read-only and never equates probe errors with absence."""
import contextlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import runtime_checks as checks


class RuntimeChecksTests(unittest.TestCase):
    def test_host_rejects_unsupported_os_architecture_python_and_os_version(self):
        for system, machine, bits, version, mac_version in (
            ('darwin', 'x86_64', 8, (3, 13), '15.6'),
            ('win32', 'ARM64', 8, (3, 13), ''),
            ('linux', 'x86_64', 4, (3, 13), ''),
            ('linux', 'x86_64', 8, (3, 9), ''),
            ('darwin', 'arm64', 8, (3, 13), '13.7'),
        ):
            with self.subTest(system=system, machine=machine, bits=bits, version=version), \
                 patch.object(checks.sys, 'platform', system), patch.object(checks.platform, 'machine', return_value=machine), \
                 patch.object(checks.struct, 'calcsize', return_value=bits), patch.object(checks.sys, 'version_info', version), \
                 patch.object(checks.platform, 'mac_ver', return_value=(mac_version, (), '')):
                with self.assertRaises(RuntimeError):
                    checks.require_host()

    def test_conda_313_x64_meets_media_requirement_without_subprocess_or_install(self):
        with patch.object(checks.sys, 'platform', 'linux'), patch.object(checks.platform, 'machine', return_value='x86_64'), \
             patch.object(checks.struct, 'calcsize', return_value=8), patch.object(checks.sys, 'version_info', (3, 13, 12)), \
             patch.object(checks.subprocess, 'run') as run:
            self.assertEqual(checks.require_host(media=True), 'linux-amd64')
        run.assert_not_called()

    def test_go_reuses_newer_stable_and_reports_old_or_wrong_architecture(self):
        for version, target, valid in [('1.26.2', 'windows/amd64', True), ('1.99.0', 'windows/amd64', True),
                                       ('1.25.9', 'windows/amd64', False), ('1.27rc1', 'windows/amd64', False),
                                       ('1.99.0', 'windows/arm64', False)]:
            with self.subTest(version=version, target=target), patch.object(checks.shutil, 'which', return_value='/native/go'), \
                 patch.object(checks.Path, 'exists', return_value=True), patch.object(checks.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, f'go version go{version} {target}\n', '')) as run, \
                 contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(bool(checks.find_go('1.26.2', 'windows-amd64')), valid)
                self.assertTrue(all(call.kwargs['env']['GOTOOLCHAIN'] == 'local' for call in run.call_args_list))

    def test_go_error_or_malformed_output_blocks_replacement_and_has_details(self):
        for result in (subprocess.CompletedProcess([], 1, '', 'missing shared library'), subprocess.CompletedProcess([], 0, 'unexpected wrapper', '')):
            with self.subTest(result=result), patch.object(checks.shutil, 'which', return_value='/native/go'), \
                 patch.object(checks.Path, 'exists', return_value=True), patch.object(checks.subprocess, 'run', return_value=result), \
                 contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaisesRegex(RuntimeError, 'not treated as missing'):
                    checks.find_go('1.26.2', 'linux-amd64')

    def test_go_can_reuse_later_path_candidate_after_older_first(self):
        def which(name, path=None):
            return '/new/go' if path == '/new' else '/old/go'
        def run(args, **kwargs):
            return subprocess.CompletedProcess([], 0, 'go version go' + ('1.99.0' if args[0] == '/new/go' else '1.25.0') + ' linux/amd64\n', '')
        with patch.object(checks.os, 'get_exec_path', return_value=['/old', '/new']), patch.object(checks.shutil, 'which', side_effect=which), \
             patch.object(checks.Path, 'exists', return_value=True), patch.object(checks.subprocess, 'run', side_effect=run), contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(checks.find_go('1.26.2', 'linux-amd64'), ('/new/go', '1.99.0'))

    def test_missing_compiler_and_old_cmake_fail_without_downloads(self):
        with patch.object(checks.shutil, 'which', return_value=None), patch.object(checks.subprocess, 'run') as run:
            with self.assertRaisesRegex(RuntimeError, 'No system installer'):
                checks.require_build_tools(('cmake',))
        run.assert_not_called()
        with patch.object(checks.shutil, 'which', return_value='/cmake'), patch.object(checks.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, 'cmake version 3.15.0\n', '')):
            with self.assertRaisesRegex(RuntimeError, r'CMake 3\.16\.3\+'):
                checks.require_build_tools(('cmake',))

    def test_node_npm_checks_native_runtime_and_executes_npm_without_shell(self):
        with tempfile.TemporaryDirectory(prefix='yandu node path ') as temporary:
            node = Path(temporary).resolve() / 'node.exe'
            node.write_text('fixture')
            npm = node.parent / 'node_modules/npm/bin/npm-cli.js'
            npm.parent.mkdir(parents=True)
            npm.write_text('fixture')
            metadata = json.dumps({'version':'24.3.1','platform':'win32','arch':'x64'})
            with patch.object(checks.shutil, 'which', return_value=str(node)), patch.object(checks, 'require_host', return_value='windows-amd64'), \
                 patch.object(checks.subprocess, 'run', side_effect=[subprocess.CompletedProcess([],0,metadata,''),subprocess.CompletedProcess([],0,'11.6.0\n','')]) as run, \
                 contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(checks.check_node_npm(), [str(node),str(npm)])
                self.assertEqual(run.call_args_list[1].args[0], [str(node),str(npm),'--version'])
                self.assertNotIn('shell', run.call_args.kwargs)

    def test_old_foreign_or_prerelease_node_is_rejected_before_npm(self):
        for version, arch in [('20.99.0','x64'),('24.0.0-rc.1','x64'),('24.3.1','arm64')]:
            with self.subTest(version=version, arch=arch), patch.object(checks.shutil, 'which', return_value='/node'), \
                 patch.object(checks, 'require_host', return_value='windows-amd64'), \
                 patch.object(checks.subprocess, 'run', return_value=subprocess.CompletedProcess([],0,json.dumps({'version':version,'platform':'win32','arch':arch}),'')) as run:
                with self.assertRaisesRegex(RuntimeError, 'Existing Node is preserved'):
                    checks.check_node_npm()
                run.assert_called_once()


if __name__ == '__main__':
    unittest.main()
