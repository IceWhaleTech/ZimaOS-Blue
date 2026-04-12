#!/usr/bin/env python3

import json
import os
import stat
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT_PATH = REPO_ROOT / "scripts" / "build_zimaos_raw.sh"


class BuildZimaOSRawScriptTest(unittest.TestCase):
    def test_stages_module_layout_and_invokes_mksquashfs(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            binary_path = tmp / "blue-linux-amd64"
            binary_path.write_text("blue-binary\n", encoding="utf-8")
            binary_path.chmod(0o755)
            dist_dir = tmp / "dist-src"
            dist_dir.mkdir()
            (dist_dir / "index.html").write_text("<html>blue</html>\n", encoding="utf-8")
            (dist_dir / "logo.svg").write_text("<svg></svg>\n", encoding="utf-8")
            (dist_dir / "stats.html").write_text("bundle stats\n", encoding="utf-8")
            (dist_dir / "index.js.gz").write_text("gzip\n", encoding="utf-8")
            (dist_dir / "index.js.br").write_text("brotli\n", encoding="utf-8")

            output_path = tmp / "dist" / "zimaos-blue-0.10.39.raw"
            staging_dir = tmp / "staging"
            fake_bin_dir = tmp / "fake-bin"
            fake_bin_dir.mkdir()
            log_path = tmp / "mksquashfs.log"
            fake_mksquashfs = fake_bin_dir / "mksquashfs"
            fake_mksquashfs.write_text(
                textwrap.dedent(
                    """\
                    #!/bin/sh
                    set -eu
                    printf '%s\n' "$@" > "${MKSQUASHFS_LOG}"
                    mkdir -p "$(dirname "$2")"
                    printf 'fake squashfs\n' > "$2"
                    """
                ),
                encoding="utf-8",
            )
            fake_mksquashfs.chmod(0o755)

            env = os.environ.copy()
            env["PATH"] = f"{fake_bin_dir}:{env['PATH']}"
            env["MKSQUASHFS_LOG"] = str(log_path)

            result = subprocess.run(
                [
                    str(SCRIPT_PATH),
                    "--binary",
                    str(binary_path),
                    "--output",
                    str(output_path),
                    "--dist-dir",
                    str(dist_dir),
                    "--staging-dir",
                    str(staging_dir),
                    "--module-name",
                    "zimaos-blue",
                    "--command-name",
                    "blue",
                    "--version",
                    "0.10.39",
                ],
                cwd=REPO_ROOT,
                env=env,
                text=True,
                capture_output=True,
            )

            self.assertEqual(
                result.returncode,
                0,
                msg=f"stdout:\n{result.stdout}\n\nstderr:\n{result.stderr}",
            )

            raw_dir = staging_dir / "raw"
            staged_binary = raw_dir / "usr" / "bin" / "blue"
            module_json = raw_dir / "usr" / "share" / "casaos" / "modules" / "zimaos-blue.json"
            module_index = (
                raw_dir / "usr" / "share" / "casaos" / "modules" / "zimaos-blue" / "index.html"
            )
            module_logo = (
                raw_dir / "usr" / "share" / "casaos" / "modules" / "zimaos-blue" / "logo.svg"
            )
            extension_release = (
                raw_dir
                / "usr"
                / "lib"
                / "extension-release.d"
                / "extension-release.zimaos-blue-0.10.39"
            )

            self.assertTrue(output_path.exists())
            self.assertEqual(output_path.read_text(encoding="utf-8"), "fake squashfs\n")

            self.assertTrue(staged_binary.exists())
            self.assertTrue(os.stat(staged_binary).st_mode & stat.S_IXUSR)
            self.assertEqual(staged_binary.read_text(encoding="utf-8"), "blue-binary\n")

            self.assertTrue(module_json.exists())
            self.assertEqual(
                json.loads(module_json.read_text(encoding="utf-8")),
                {
                    "name": "zimaos-blue",
                    "ui": {
                        "name": "zimaos-blue",
                        "title": {"en_us": "ZimaOS Blue"},
                        "prefetch": True,
                        "show": True,
                        "entry": "/modules/zimaos-blue/index.html",
                        "icon": "/modules/zimaos-blue/logo.svg",
                        "description": "ZimaOS Blue",
                        "formality": {
                            "type": "newtab",
                            "props": {
                                "width": "100vh",
                                "height": "100vh",
                                "hasModalCard": True,
                                "animation": "zoom-in",
                            },
                        },
                    },
                    "version": "0.10.39",
                },
            )
            self.assertEqual(module_index.read_text(encoding="utf-8"), "<html>blue</html>\n")
            self.assertEqual(module_logo.read_text(encoding="utf-8"), "<svg></svg>\n")
            self.assertFalse((raw_dir / "usr" / "share" / "casaos" / "modules" / "zimaos-blue" / "stats.html").exists())
            self.assertFalse((raw_dir / "usr" / "share" / "casaos" / "modules" / "zimaos-blue" / "index.js.gz").exists())
            self.assertFalse((raw_dir / "usr" / "share" / "casaos" / "modules" / "zimaos-blue" / "index.js.br").exists())

            self.assertTrue(extension_release.exists())
            self.assertEqual(extension_release.read_text(encoding="utf-8"), "ID=_any\n")

            self.assertEqual(
                log_path.read_text(encoding="utf-8").splitlines(),
                [str(raw_dir), str(output_path), "-noappend"],
            )


if __name__ == "__main__":
    unittest.main()
