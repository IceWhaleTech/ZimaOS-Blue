import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent
RELEASE_WORKFLOW = REPO_ROOT / ".github" / "workflows" / "release.yml"
SERVER_MAKEFILE = REPO_ROOT / "server" / "Makefile"


class ReleasePackagingCleanupTest(unittest.TestCase):
    def test_release_workflow_uploads_only_named_cli_archives(self):
        content = RELEASE_WORKFLOW.read_text(encoding="utf-8")

        self.assertIn('rm -f "${DIST_TAR}"', content)
        self.assertIn("server/bin/blue-*.tar.gz", content)
        self.assertNotIn("server/bin/*.tar.gz", content)
        self.assertIsNone(re.search(r"[.]sha256\b", content))

    def test_server_makefile_cleans_build_dist_archive_after_launcher_build(self):
        content = SERVER_MAKEFILE.read_text(encoding="utf-8")

        self.assertIn(
            "@rm -f $(BUILD_DIR)/dist.tar $(BUILD_DIR)/dist.tar.gz",
            content,
        )
        self.assertIn(
            "@rm -f $(BUILD_DIR)/dist.tar $(BUILD_DIR)/dist.tar.gz $(LAUNCHER_DIR)/bluecli $(LAUNCHER_DIR)/bluecli.gz $(LAUNCHER_DIR)/dist.tar $(LAUNCHER_DIR)/dist.tar.gz",
            content,
        )


if __name__ == "__main__":
    unittest.main()
