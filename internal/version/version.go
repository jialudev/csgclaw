package version

import (
	"runtime/debug"
	"strings"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func Current() string {
	if version := strings.TrimSpace(Version); version != "" && version != "dev" {
		return version
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if version := strings.TrimSpace(info.Main.Version); version != "" && version != "(devel)" {
			return version
		}
	}

	return "dev"
}
