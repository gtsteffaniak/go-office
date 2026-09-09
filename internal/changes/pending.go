package changes

// FileName is the coauthoring changes blob file x2t reads when m_bFromChanges is true.
const FileName = "changes0.json"

// HasPending reports whether cacheDir has non-empty coauthoring change blobs.
func HasPending(cacheDir string) bool {
	pending := false
	_ = WithDirLock(cacheDir, func() error {
		blobs, err := readBlobs(changesPath(cacheDir))
		if err != nil || len(blobs) == 0 {
			return err
		}
		pending = true
		return nil
	})
	return pending
}
