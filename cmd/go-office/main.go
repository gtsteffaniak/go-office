package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	office "github.com/quantumx-apps/go-office"
	"github.com/quantumx-apps/go-office/internal/debuglog"
	"github.com/quantumx-apps/go-office/internal/demo"
	"github.com/quantumx-apps/go-office/internal/home"
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
	dataDir := flag.String("data", ".", "document root (repo root; contains sample-files/)")
	samplesDir := flag.String("samples", demo.DefaultSamplesDir, "directory of sample documents relative to -data")
	addr := flag.String("addr", ":8080", "listen address")
	basePath := flag.String("base", office.DefaultBasePath, "document server URL prefix (editor assets)")
	apiBase := flag.String("api-base", home.DefaultAPIBasePath, "API URL prefix (config, callback, file)")
	jwtSecret := flag.String("jwt", "", "JWT secret for editor config signing")
	version := flag.String("version", "", "protocol version (default: read from assets/VERSION)")
	demoUI := flag.Bool("demo", true, "serve local demo editor UI at {base}/demo/")
	debugFlag := flag.Bool("debug", false, "enable verbose logging (on by default with -demo)")
	publicOrigin := flag.String("public", "", "public origin for document URLs (default: http://{addr})")
	flag.Parse()

	if *assetDir == "" {
		log.Fatal("assets required: run `make build`")
	}
	if v, err := office.ReadAssetVersion(*assetDir); err == nil && *version == "" {
		*version = v
	}
	if *version == "" {
		*version = "0.0.0-dev"
	}

	samplesPath := filepath.Join(*dataDir, filepath.FromSlash(*samplesDir))
	if *demoUI {
		if fi, err := os.Stat(samplesPath); err != nil || !fi.IsDir() {
			log.Fatalf("samples directory not found: %s (%v)", samplesPath, err)
		}
	}

	debug := debuglog.Enabled(*debugFlag) || *demoUI
	logger := debuglog.NewLogger(debug)
	slog.SetDefault(logger)

	store := &localStorage{root: *dataDir}
	srv, err := office.New(store, office.Options{
		AssetDir:        *assetDir,
		BasePath:        *basePath,
		JWTSecret:       []byte(*jwtSecret),
		ProtocolVersion: *version,
		Debug:           debug,
		Logger:          logger,
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	defer srv.Close()

	origin := *publicOrigin
	if origin == "" {
		host, port, _ := net.SplitHostPort(*addr)
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "localhost"
		}
		if port == "" {
			port = "8080"
		}
		origin = fmt.Sprintf("http://%s", net.JoinHostPort(host, port))
	}

	if *demoUI {
		if err := demo.Attach(srv, store, demo.Options{
			PublicOrigin: origin,
			DataRoot:     *dataDir,
			SamplesDir:   *samplesDir,
			APIBasePath:  *apiBase,
			Logger:       logger,
		}); err != nil {
			log.Fatalf("demo: %v", err)
		}
	}

	homeHandler, err := home.New(home.Options{
		Origin:     origin,
		OfficeBase: srv.BasePath(),
		APIBase:    *apiBase,
		SamplesDir: *samplesDir,
	})
	if err != nil {
		log.Fatalf("home: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", homeHandler)
	mux.Handle("/", srv.Handler())

	officeBase := strings.TrimSuffix(origin, "/") + srv.BasePath()
	apiBasePath := strings.TrimSuffix(*apiBase, "/")
	log.Printf("go-office listening on %s", *addr)
	if debug {
		log.Printf("  debug:   enabled")
	}
	log.Printf("  site:    %s/", strings.TrimSuffix(origin, "/"))
	log.Printf("  health:  %s/health", officeBase)
	if *demoUI {
		log.Printf("  demo:    %s/demo/", officeBase)
		log.Printf("  api:     %s/demo/config", strings.TrimSuffix(origin, "/")+apiBasePath)
		log.Printf("  samples: %s", samplesPath)
	}
	log.Printf("  assets:  %s", *assetDir)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

var _ office.Storage = (*localStorage)(nil)
