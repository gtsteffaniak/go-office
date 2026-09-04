//go:build !linux

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "fetch-assets is Linux-only (downloads and unpacks the Euro-Office .deb).")
	fmt.Fprintln(os.Stderr, "Run: make build  (uses Docker on macOS/Windows when Docker is installed)")
	os.Exit(1)
}
