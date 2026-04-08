import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent
RELEASE_WORKFLOW = REPO_ROOT / ".github" / "workflows" / "release.yml"
ESPEAK_FILES = [
    REPO_ROOT / "server" / "internal" / "tts" / "espeak_ng.go",
    REPO_ROOT / "server" / "internal" / "tts" / "g2p_en_espeak.go",
]


class ReleaseEspeakLinkingTest(unittest.TestCase):
    def test_release_workflow_disables_libpcaudio_for_espeak_build(self):
        text = RELEASE_WORKFLOW.read_text(encoding="utf-8")
        self.assertIn(
            "-DUSE_LIBPCAUDIO=OFF",
            text,
            "release workflow should disable libpcaudio when building espeak-ng to avoid runner-dependent link errors",
        )

    def test_linux_and_darwin_cgo_flags_do_not_link_nonexistent_libsonic_archive(self):
        violations = []
        for path in ESPEAK_FILES:
            text = path.read_text(encoding="utf-8")
            for line in text.splitlines():
                if line.startswith("#cgo linux LDFLAGS:") and "-lsonic" in line:
                    violations.append(f"{path.name} linux LDFLAGS still contain -lsonic")
                if line.startswith("#cgo darwin LDFLAGS:") and "-lsonic" in line:
                    violations.append(f"{path.name} darwin LDFLAGS still contain -lsonic")

        self.assertFalse(violations, "; ".join(violations))


if __name__ == "__main__":
    unittest.main()
