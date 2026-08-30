package office

import (
	"context"
	"io"
	"time"
)

// FileInfo describes a document known to the host application.
type FileInfo struct {
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
}

// Storage is implemented by the host application (e.g. FileBrowser VFS).
// The library uses it for direct reads/writes instead of HTTP callbacks when possible.
type Storage interface {
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	Save(ctx context.Context, path string, r io.Reader) error
	Stat(ctx context.Context, path string) (FileInfo, error)
}
