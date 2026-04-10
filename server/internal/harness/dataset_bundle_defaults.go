package harness

import "strings"

const defaultPinchBenchDatasetBundleGitHubPath = "harness/datasets/pinchbench"

func DefaultDatasetBundleGitHubPathForRepo(owner, repo string) string {
	if strings.EqualFold(strings.TrimSpace(owner), "IceWhaleTech") &&
		strings.EqualFold(strings.TrimSpace(repo), "ZimaOS-Blue") {
		return defaultPinchBenchDatasetBundleGitHubPath
	}
	return ""
}
