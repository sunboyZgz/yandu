"""Verify dependency-only preparation never starts or probes storage services."""
import importlib.util
import io
import contextlib
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("storage_dev", Path(__file__).with_name("storage-dev.py"))
storage = importlib.util.module_from_spec(spec)
spec.loader.exec_module(storage)


class StoragePreparationTests(unittest.TestCase):
    def test_prepare_only_builds_local_tools_without_ports_backend_or_agent(self):
        for no_media in (True, False):
            with self.subTest(no_media=no_media), tempfile.TemporaryDirectory(prefix="yandu storage setup ") as temporary:
                root = Path(temporary).resolve()
                args = ["storage-dev.py", "--prepare-only", "--state-dir", "state & spaces", "--bin-dir", "bin spaces"]
                if no_media:
                    args.append("--no-media")
                with patch.object(storage, "ROOT", root), patch.object(sys, "argv", args), \
                     patch.object(storage, "host_target", return_value="windows-amd64"), \
                     patch.object(storage, "check_source_environment"), \
                     patch.object(storage, "prepare_state") as state, patch.object(storage, "build") as build, \
                     patch.object(storage, "ensure_ports_free") as ports, patch.object(storage.socket, "socket") as socket, \
                     patch.object(storage, "prepare_backend") as backend, patch.object(storage, "run_agent") as agent, \
                     contextlib.redirect_stdout(io.StringIO()):
                    storage.main()
                build.assert_called_once_with(root, root / "bin spaces", "windows-amd64", tools_config=root / "state & spaces/tools.json", role="local", no_media=no_media, overrides={})
                state.assert_called_once()
                for operation in (ports, socket, backend, agent):
                    operation.assert_not_called()


if __name__ == "__main__":
    unittest.main()
