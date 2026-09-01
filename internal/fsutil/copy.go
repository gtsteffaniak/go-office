package fsutil

import (
	"io"
	"os"
)

// CopyFile copies src to dst with mode 0644.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// CopyExecutable copies src to dst with mode 0755.
func CopyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// LinkExecutable places src at dst for isolated x2t runs. A hard link keeps the
// same ELF inode while making /proc/self/exe resolve under the run directory;
// fall back to a byte copy when hard links are unsupported.
func LinkExecutable(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return os.Chmod(dst, 0o755)
	}
	return CopyExecutable(src, dst)
}
