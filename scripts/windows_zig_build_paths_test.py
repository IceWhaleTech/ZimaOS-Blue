import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent
BUILD_BAT = REPO_ROOT / "build.bat"
SERVER_MAKEFILE = REPO_ROOT / "server" / "Makefile"
WINDOWS_LIB_BUILD = REPO_ROOT / "server" / "build-windows-lib.sh"
WINDOWS_TAURI_BUILD = REPO_ROOT / "tauri-app" / "src-tauri" / "nsis" / "build-all.ps1"


class WindowsZigBuildPathsTest(unittest.TestCase):
    def test_windows_batch_build_uses_zig_toolchain(self):
        content = BUILD_BAT.read_text(encoding="utf-8")

        self.assertIn('set "CC=zig cc -target x86_64-windows-gnu"', content)
        self.assertIn('set "CXX=zig c++ -target x86_64-windows-gnu"', content)

    def test_server_makefile_windows_cross_build_uses_zig_toolchain(self):
        content = SERVER_MAKEFILE.read_text(encoding="utf-8")

        self.assertIn('ZIG_CC_WINDOWS_AMD64=zig cc -target x86_64-windows-gnu', content)
        self.assertIn('ZIG_CXX_WINDOWS_AMD64=zig c++ -target x86_64-windows-gnu', content)
        self.assertIn('CC="$(ZIG_CC_WINDOWS_AMD64)" CXX="$(ZIG_CXX_WINDOWS_AMD64)" GOOS=windows GOARCH=amd64 go build', content)

    def test_windows_lib_builder_uses_zig_toolchain(self):
        content = WINDOWS_LIB_BUILD.read_text(encoding="utf-8")

        self.assertIn('export CC="zig cc -target x86_64-windows-gnu"', content)
        self.assertIn('export CXX="zig c++ -target x86_64-windows-gnu"', content)

    def test_windows_tauri_builder_uses_zig_toolchain(self):
        content = WINDOWS_TAURI_BUILD.read_text(encoding="utf-8")

        self.assertIn('$env:CC = "zig cc -target x86_64-windows-gnu"', content)
        self.assertIn('$env:CXX = "zig c++ -target x86_64-windows-gnu"', content)


if __name__ == "__main__":
    unittest.main()
