package version

import (
	"strings"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func Current() string {
	if version := strings.TrimSpace(Version); version != "" {
		return version
	}
	return "dev"
}
