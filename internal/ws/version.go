package ws

import "strings"

// BuildInfo maps assets/VERSION to the build fields expected by sdkjs.
// sdkjs embeds "9.3.4 (build:0)" while VERSION may be "9.3.4-hotfix.1".
type BuildInfo struct {
	Release      string
	BuildVersion string
	BuildNumber  int
}

func ParseBuild(release string) BuildInfo {
	if release == "" {
		release = "0.0.0"
	}
	info := BuildInfo{
		Release:     release,
		BuildNumber: 0,
	}
	base := release
	if i := strings.IndexByte(base, '-'); i >= 0 {
		base = base[:i]
	}
	parts := strings.Split(base, ".")
	if len(parts) >= 3 {
		info.BuildVersion = parts[0] + "." + parts[1] + "." + parts[2]
	} else {
		info.BuildVersion = base
	}
	return info
}
