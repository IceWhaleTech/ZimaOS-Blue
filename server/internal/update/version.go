package update

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string // alpha, beta, rc
	PreNum     int    // alpha.1 -> 1
}

var versionRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-(alpha|beta|rc)(?:\.(\d+))?)?$`)

// ParseVersion parses a semantic version string
func ParseVersion(s string) (*Version, error) {
	matches := versionRegex.FindStringSubmatch(strings.ToLower(s))
	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", s)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	v := &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
	}

	if matches[5] != "" {
		v.PreNum, _ = strconv.Atoi(matches[5])
	}

	return v, nil
}

// String returns the version string
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += fmt.Sprintf("-%s", v.Prerelease)
		if v.PreNum > 0 {
			s += fmt.Sprintf(".%d", v.PreNum)
		}
	}
	return s
}

// Compare compares two versions. Returns -1 if v < other, 0 if equal, 1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return compareInt(v.Patch, other.Patch)
	}

	// Prerelease comparison: no prerelease > prerelease
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		return comparePrerelease(v.Prerelease, other.Prerelease)
	}

	return compareInt(v.PreNum, other.PreNum)
}

// Channel returns the release channel based on prerelease tag
func (v *Version) Channel() string {
	switch v.Prerelease {
	case "alpha":
		return ChannelAlpha
	case "beta":
		return ChannelBeta
	default:
		return ChannelStable
	}
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func comparePrerelease(a, b string) int {
	order := map[string]int{"alpha": 1, "beta": 2, "rc": 3}
	return compareInt(order[a], order[b])
}

// IsNewerThan returns true if v is newer than other
func (v *Version) IsNewerThan(other *Version) bool {
	return v.Compare(other) > 0
}
