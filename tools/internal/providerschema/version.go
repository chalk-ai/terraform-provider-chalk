package providerschema

import (
	"fmt"
	"strconv"
)

// Version is a parsed release version.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses the repository's strict release version format.
func ParseVersion(value string) (Version, error) {
	matches := releaseVersionPattern.FindStringSubmatch(value)
	if matches == nil {
		return Version{}, fmt.Errorf("version %q must match vMAJOR.MINOR.PATCH", value)
	}
	parts := [3]int{}
	for i := range parts {
		parsed, err := strconv.Atoi(matches[i+1])
		if err != nil {
			return Version{}, fmt.Errorf("parsing version %q: %w", value, err)
		}
		parts[i] = parsed
	}
	return Version{Major: parts[0], Minor: parts[1], Patch: parts[2]}, nil
}

// Compare returns -1, 0, or 1 when v is less than, equal to, or greater than
// other.
func (v Version) Compare(other Version) int {
	left := [...]int{v.Major, v.Minor, v.Patch}
	right := [...]int{other.Major, other.Minor, other.Patch}
	for i := range left {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	return 0
}
