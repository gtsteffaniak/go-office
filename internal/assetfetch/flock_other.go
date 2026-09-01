//go:build !linux

package assetfetch

func withFetchLock(_ string, fn func() error) error {
	return fn()
}
