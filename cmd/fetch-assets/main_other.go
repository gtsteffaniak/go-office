//go:build !linux

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "fetch-assets is Linux-only (downloads and unpacks the Euro-Office .deb).")
	fmt.Fprintln(os.Stderr, "Use WSL, a Linux VM, or GitHub Actions CI to populate ./assets/.")
	os.Exit(1)
}
