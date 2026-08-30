package main

import (
	"context"
	"flag"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	office "github.com/quantumx-apps/go-office"
)

type localStorage struct {
	root string
}

func (s *localStorage) Open(_ context.Context, path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.root, filepath.FromSlash(path)))
}

func (s *localStorage) Save(_ context.Context, path string, r io.Reader) error {
	full := filepath.Join(s.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *localStorage) Stat(_ context.Context, path string) (office.FileInfo, error) {
	full := filepath.Join(s.root, filepath.FromSlash(path))
	fi, err := os.Stat(full)
	if err != nil {
		return office.FileInfo{}, err
	}
	return office.FileInfo{
		Path:    path,
		Name:    fi.Name(),
		Size:    fi.Size(),
		ModTime: fi.ModTime().UTC(),
	}, nil
}

func main() {
	assetDir := flag.String("assets", office.AssetDirFromEnv(""), "path to Euro-Office assets (web-apps/, sdkjs/)")
	dataDir := flag.String("data", ".", "local document root for Storage")
	addr := flag.String("addr", ":8080", "listen address")
	basePath := flag.String("base", office.DefaultBasePath, "URL prefix (use app subpath + /api/office for reverse-proxy mounts)")
	jwtSecret := flag.String("jwt", "", "JWT secret for editor config signing")
	version := flag.String("version", "0.0.0-dev", "protocol version reported to sdkjs")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}

	if *assetDir != "" {
		if v, err := office.ReadAssetVersion(*assetDir); err == nil {
			*version = v
		}
	}

	srv, err := office.New(&localStorage{root: *dataDir}, office.Options{
		AssetDir:        *assetDir,
		BasePath:        *basePath,
		JWTSecret:       []byte(*jwtSecret),
		ProtocolVersion: *version,
		Logger:          slog.Default(),
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	defer srv.Close()

	log.Printf("go-office listening on %s (base=%s assets=%q)", *addr, srv.BasePath(), *assetDir)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}

// Ensure localStorage satisfies office.Storage at compile time.
var _ office.Storage = (*localStorage)(nil)
