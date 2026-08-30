package assetfetch

import (
	"fmt"
	"os"
	"os/exec"
)

// ExtractDebData unpacks a .deb into dest.
// Uses dpkg-deb on Linux; falls back to parsing the ar archive and system tar.
func ExtractDebData(debPath, dest string) error {
	if err := RequireLinux(); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if dpkg, err := exec.LookPath("dpkg-deb"); err == nil {
		cmd := exec.Command(dpkg, "-x", debPath, dest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("dpkg-deb -x: %w", err)
		}
		return nil
	}
	return extractDebDataTar(debPath, dest)
}
