import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import windows_codex_release_assets as codex_assets


class StubResponse:
    def __init__(self, payload: bytes):
        self.payload = payload

    def read(self, amt: int = -1) -> bytes:
        if amt < 0:
            amt = len(self.payload)
        chunk = self.payload[:amt]
        self.payload = self.payload[amt:]
        return chunk

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False


class WindowsCodexReleaseAssetsTest(unittest.TestCase):
    def test_windows_codex_asset_names_match_runtime_expectations(self):
        self.assertEqual(
            codex_assets.codex_asset_name_for_windows_arch("amd64"),
            "codex-x86_64-pc-windows-msvc.exe",
        )
        self.assertEqual(
            codex_assets.codex_asset_name_for_windows_arch("arm64"),
            "codex-aarch64-pc-windows-msvc.exe",
        )

    def test_stage_prefers_local_source_dir_when_assets_exist(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            source_dir = root / "source"
            output_dir = root / "out"
            source_dir.mkdir(parents=True, exist_ok=True)

            amd64_asset = source_dir / "codex-x86_64-pc-windows-msvc.exe"
            arm64_asset = source_dir / "codex-aarch64-pc-windows-msvc.exe"
            amd64_asset.write_bytes(b"amd64-local")
            arm64_asset.write_bytes(b"arm64-local")

            downloaded_urls = []

            def fail_download(url: str, timeout: float):
                downloaded_urls.append((url, timeout))
                raise AssertionError("download should not be used when local assets exist")

            staged = codex_assets.stage_windows_codex_assets(
                output_dir=output_dir,
                archs=["amd64", "arm64"],
                source_dir=source_dir,
                download_url_base="https://example.invalid/latest/download",
                timeout_seconds=30.0,
                urlopen=fail_download,
            )

            self.assertEqual(
                [item.asset_name for item in staged],
                [
                    "codex-x86_64-pc-windows-msvc.exe",
                    "codex-aarch64-pc-windows-msvc.exe",
                ],
            )
            self.assertEqual((output_dir / amd64_asset.name).read_bytes(), b"amd64-local")
            self.assertEqual((output_dir / arm64_asset.name).read_bytes(), b"arm64-local")
            self.assertEqual(downloaded_urls, [])

    def test_stage_downloads_missing_assets_with_expected_urls(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            output_dir = root / "out"
            payloads = {
                "https://downloads.example/latest/download/codex-x86_64-pc-windows-msvc.exe": b"amd64-bin",
                "https://downloads.example/latest/download/codex-aarch64-pc-windows-msvc.exe": b"arm64-bin",
            }
            requested = []

            def fake_urlopen(url: str, timeout: float):
                requested.append((url, timeout))
                return StubResponse(payloads[url])

            staged = codex_assets.stage_windows_codex_assets(
                output_dir=output_dir,
                archs=["amd64", "arm64"],
                source_dir=None,
                download_url_base="https://downloads.example/latest/download",
                timeout_seconds=42.0,
                urlopen=fake_urlopen,
            )

            self.assertEqual(
                [item.download_url for item in staged],
                [
                    "https://downloads.example/latest/download/codex-x86_64-pc-windows-msvc.exe",
                    "https://downloads.example/latest/download/codex-aarch64-pc-windows-msvc.exe",
                ],
            )
            self.assertEqual(
                requested,
                [
                    ("https://downloads.example/latest/download/codex-x86_64-pc-windows-msvc.exe", 42.0),
                    ("https://downloads.example/latest/download/codex-aarch64-pc-windows-msvc.exe", 42.0),
                ],
            )
            self.assertEqual((output_dir / "codex-x86_64-pc-windows-msvc.exe").read_bytes(), b"amd64-bin")
            self.assertEqual((output_dir / "codex-aarch64-pc-windows-msvc.exe").read_bytes(), b"arm64-bin")

    def test_stage_rejects_unknown_windows_arch(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            with self.assertRaises(ValueError):
                codex_assets.stage_windows_codex_assets(
                    output_dir=Path(tmp_dir) / "out",
                    archs=["386"],
                    source_dir=None,
                    download_url_base="https://downloads.example/latest/download",
                    timeout_seconds=10.0,
                    urlopen=lambda url, timeout: StubResponse(b""),
                )


if __name__ == "__main__":
    unittest.main()
