package assetfetch

import (
	"fmt"
	"runtime"
)

// RequireLinux returns an error when not running on Linux.
func RequireLinux() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("go-office assets require Linux (got %s/%s)", runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

// DebArch returns the Debian package arch for this Linux host (amd64 or arm64).
func DebArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	default:
		return "amd64"
	}
}
