package toolchain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var stableRuntimeVersion = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:\+[0-9A-Za-z.-]+)?$`)

func versionFromOutput(name, line string, s spec) (string, error) {
	line = strings.TrimSpace(line)
	if s.MinimumVersion == "" {
		if !regexp.MustCompile(`(?:^|\s)v?` + regexp.QuoteMeta(s.Version) + `(?:[\s+\-]|$)`).MatchString(line) {
			return "", fmt.Errorf("%s requires locked version %s", name, s.Version)
		}
		return s.Version, nil
	}
	actual := stableRuntimeVersion.FindStringSubmatch(line)
	minimum := stableRuntimeVersion.FindStringSubmatch(s.MinimumVersion)
	if actual == nil || minimum == nil {
		return "", fmt.Errorf("%s requires stable version >= %s (got %q)", name, s.MinimumVersion, line)
	}
	for i := 1; i <= 3; i++ {
		a, e := strconv.ParseUint(actual[i], 10, 64)
		b, minimumError := strconv.ParseUint(minimum[i], 10, 64)
		if e != nil || minimumError != nil {
			return "", fmt.Errorf("invalid %s runtime version %q", name, line)
		}
		if a < b {
			return "", fmt.Errorf("%s requires version >= %s (got %s)", name, s.MinimumVersion, line)
		}
		if a > b {
			break
		}
	}
	return strings.TrimPrefix(line, "v"), nil
}
