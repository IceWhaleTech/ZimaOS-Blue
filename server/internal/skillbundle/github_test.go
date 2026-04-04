package skillbundle

import "testing"

func TestGitHubRawURLCandidatesReturnsFourMirrors(t *testing.T) {
	candidates := GitHubRawURLCandidates("demo", "skills", "main", "skills/git/SKILL.md")
	if len(candidates) != 4 {
		t.Fatalf("candidate count = %d, want 4", len(candidates))
	}
	want := []string{
		"https://raw.githubusercontent.com/demo/skills/main/skills/git/SKILL.md",
		"https://raw.gitmirror.com/demo/skills/main/skills/git/SKILL.md",
		"https://cdn.jsdelivr.net/gh/demo/skills@main/skills/git/SKILL.md",
		"https://ghproxy.com/https://raw.githubusercontent.com/demo/skills/main/skills/git/SKILL.md",
	}
	for i := range want {
		if candidates[i] != want[i] {
			t.Fatalf("candidate[%d] = %q, want %q", i, candidates[i], want[i])
		}
	}
}
