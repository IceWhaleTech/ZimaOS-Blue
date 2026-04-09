import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent
RELEASE_WORKFLOW = REPO_ROOT / ".github" / "workflows" / "release.yml"


class ReleaseWorkflowCodexAssetsTest(unittest.TestCase):
    def test_release_workflow_does_not_stage_windows_codex_assets(self):
        content = RELEASE_WORKFLOW.read_text(encoding="utf-8")

        self.assertNotIn("build-windows-codex-assets:", content)
        self.assertNotIn("blue-codex-windows-assets", content)
        self.assertNotIn("scripts/windows_codex_release_assets.py", content)

    def test_release_notes_do_not_include_windows_codex_asset_links(self):
        content = RELEASE_WORKFLOW.read_text(encoding="utf-8")

        self.assertNotIn("codex-x86_64-pc-windows-msvc.exe", content)
        self.assertNotIn("codex-aarch64-pc-windows-msvc.exe", content)


if __name__ == "__main__":
    unittest.main()
