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
	PreNum     int    // alpha1 -> 1, or bare revision like 0.4.2-1 -> 1
}

var tagsRE = regexp.MustCompile(`^([a-zA-Z]+)?(\d+)?$`)

// ParseVersion parses a version string.
// Supports: 1.2.3, v1.2.3, 0.4.2-alpha1, 0.4.2-beta.1, 0.4.2-rc1, 0.4.2-1
func ParseVersion(s string) (*Version, error) {
	s = strings.TrimPrefix(strings.ToLower(s), "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid version format: %s", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", s)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", s)
	}

	v := &Version{Major: major, Minor: minor}

	// parts[2] may be "3", "3-alpha1", "3-beta.1", "3-1"
	tagParts := strings.SplitN(parts[2], "-", 2)
	patch, err := strconv.Atoi(tagParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", s)
	}
	v.Patch = patch

	if len(tagParts) > 1 {
		seqs := tagsRE.FindStringSubmatch(tagParts[1])
		switch len(seqs) {
		case 2:
			// either pure tag "alpha" or pure number "1"
			if seqs[1] != "" {
				v.Prerelease = seqs[1]
			} else {
				v.PreNum, _ = strconv.Atoi(seqs[0])
			}
		case 3:
			v.Prerelease = seqs[1]
			v.PreNum, _ = strconv.Atoi(seqs[2])
		}
	}

	return v, nil
}

// String returns the version string
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += fmt.Sprintf("-%s", v.Prerelease)
		if v.PreNum > 0 {
			s += fmt.Sprintf("%d", v.PreNum)
		}
	} else if v.PreNum > 0 {
		s += fmt.Sprintf("-%d", v.PreNum)
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
	oa, aKnown := order[a]
	ob, bKnown := order[b]
	if aKnown && bKnown {
		return compareInt(oa, ob)
	}
	// Unknown tags are considered higher than known tags
	if !aKnown && bKnown {
		return 1
	}
	if aKnown && !bKnown {
		return -1
	}
	return strings.Compare(a, b)
}

// IsNewerThan returns true if v is newer than other
func (v *Version) IsNewerThan(other *Version) bool {
	return v.Compare(other) > 0
}

// IsNewerVersionString compares two version strings, returns true if target > current.
func IsNewerVersionString(current, target string) bool {
	cv, err := ParseVersion(current)
	if err != nil {
		return false
	}
	tv, err := ParseVersion(target)
	if err != nil {
		return false
	}
	return tv.IsNewerThan(cv)
}
