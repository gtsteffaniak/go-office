package demo

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/singleflight"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

const thumbnailGenTimeout = 3 * time.Minute

type thumbnailCoordinator struct {
	sf singleflight.Group
}

func newThumbnailCoordinator() *thumbnailCoordinator {
	return &thumbnailCoordinator{}
}

func (h *Handler) thumbnailJPEG(ctx context.Context, origin string, file string, req office.ConverterRequest) ([]byte, error) {
	cacheName := office.ConvCacheDirName(req.Key, req.OutputType)
	outPath := filepath.Join(h.office.CacheDir(), cacheName, office.ConvOutputBasename("jpg", req.Thumbnail))
	if raw, err := os.ReadFile(outPath); err == nil && len(raw) > 0 {
		return raw, nil
	}

	raw, err, _ := h.thumbs.sf.Do(file, func() (any, error) {
		if cached, readErr := os.ReadFile(outPath); readErr == nil && len(cached) > 0 {
			return cached, nil
		}
		genCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), thumbnailGenTimeout)
		defer cancel()
		if _, runErr := h.office.RunConverter(genCtx, origin, req); runErr != nil {
			return nil, runErr
		}
		return os.ReadFile(outPath)
	})
	if err != nil {
		return nil, err
	}
	jpeg, ok := raw.([]byte)
	if !ok || len(jpeg) == 0 {
		return nil, os.ErrNotExist
	}
	return jpeg, nil
}
