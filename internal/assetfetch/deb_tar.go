package assetfetch

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const arMagic = "!<arch>\n"

func extractDebDataTar(debPath, dest string) error {
	dataTar, err := extractDataTarMember(debPath)
	if err != nil {
		return err
	}
	defer os.Remove(dataTar)
	return runTarExtract(dataTar, dest)
}

func extractDataTarMember(debPath string) (string, error) {
	f, err := os.Open(debPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	magic := make([]byte, len(arMagic))
	if _, err := io.ReadFull(f, magic); err != nil {
		return "", fmt.Errorf("read deb: %w", err)
	}
	if string(magic) != arMagic {
		return "", fmt.Errorf("%s: not a deb/ar archive", debPath)
	}

	for {
		hdr := make([]byte, 60)
		if _, err := io.ReadFull(f, hdr); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		name := strings.TrimSpace(string(hdr[0:16]))
		size, err := strconv.Atoi(strings.TrimSpace(string(hdr[48:58])))
		if err != nil {
			return "", fmt.Errorf("deb member %q: bad size", name)
		}

		data := make([]byte, size)
		if _, err := io.ReadFull(f, data); err != nil {
			return "", err
		}
		if size%2 == 1 {
			var pad [1]byte
			_, _ = io.ReadFull(f, pad[:])
		}

		if strings.HasPrefix(name, "data.tar") {
			tmp, err := os.CreateTemp("", "go-office-data-*.tar*")
			if err != nil {
				return "", err
			}
			path := tmp.Name()
			if _, err := tmp.Write(data); err != nil {
				tmp.Close()
				os.Remove(path)
				return "", err
			}
			if err := tmp.Close(); err != nil {
				os.Remove(path)
				return "", err
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("%s: data.tar member not found", debPath)
}

func runTarExtract(archivePath, dest string) error {
	tarBin, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("tar not found in PATH (install tar or dpkg-deb): %w", err)
	}
	cmd := exec.Command(tarBin, "-xf", archivePath, "-C", dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}
	return nil
}
