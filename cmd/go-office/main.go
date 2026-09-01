package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	office "github.com/quantumx-apps/go-office/pkg/office"
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
	cfg := parseRunConfig()

	ctx := context.Background()
	debug := debuglog.Enabled(cfg.Debug)
	logger := debuglog.NewLogger(debug)
	slog.SetDefault(logger)

	var bundle office.AssetBundle
	var err error
	if cfg.SkipAssetFetch {
		var ok bool
		bundle, ok, err = office.DiscoverAssets(office.AssetOptions{Dir: cfg.AssetDir, Logger: logger})
		if err != nil {
			log.Fatalf("assets: %v", err)
		}
		if !ok {
			log.Fatal("assets not found: set OFFICE_ASSETS, run `make build`, or remove -skip-asset-fetch")
		}
	} else {
		bundle, err = office.EnsureAssets(ctx, office.EnsureAssetsOptions{
			AssetOptions: office.AssetOptions{Dir: cfg.AssetDir, Logger: logger},
		})
		if err != nil {
			log.Fatalf("assets: %v", err)
		}
	}
	cfg.AssetDir = bundle.Dir
	if cfg.Version == "" {
		cfg.Version = bundle.Version
	}
	if cfg.Version == "" {
		if v, readErr := office.ReadAssetVersion(cfg.AssetDir); readErr == nil {
			cfg.Version = v
		}
	}
	if cfg.Version == "" {
		cfg.Version = "0.0.0-dev"
	}

	samplesPath := filepath.Join(cfg.DataDir, filepath.FromSlash(cfg.SamplesDir))
	if cfg.samplesEnabled() {
		if fi, err := os.Stat(samplesPath); err != nil || !fi.IsDir() {
			log.Fatalf("samples directory not found: %s (%v) — use OFFICE_DISABLE_SAMPLES=1 to run without demo", samplesPath, err)
		}
	}

	store := &localStorage{root: cfg.DataDir}
	srv, err := office.New(store, office.Options{
		AssetDir:        cfg.AssetDir,
		BasePath:        cfg.BasePath,
		JWTSecret:       []byte(cfg.JWTSecret),
		ProtocolVersion: cfg.Version,
		PublicOrigin:    cfg.PublicOrigin,
		ConvertLimit:    cfg.ConvertLimit,
		PollHold:        cfg.PollHold,
		SaveDelay:       cfg.SaveDelay,
		Debug:           debug,
		Logger:          logger,
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}
	defer srv.Close()

	origin := cfg.publicOrigin()

	if cfg.samplesEnabled() {
		if err := demo.Attach(srv, store, demo.Options{
			PublicOrigin: cfg.PublicOrigin,
			DataRoot:     cfg.DataDir,
			SamplesDir:   cfg.SamplesDir,
			APIBasePath:  cfg.APIBase,
			Logger:       logger,
		}); err != nil {
			log.Fatalf("demo: %v", err)
		}
	}

	logoPath := home.DefaultLogoPath
	homeHandler, err := home.New(home.Options{
		OfficeBase: srv.BasePath(),
		APIBase:    cfg.APIBase,
		SamplesDir: cfg.SamplesDir,
		Version:    cfg.Version,
		LogoURL:    office.URLPath(srv.BasePath(), logoPath),
		SamplesOn:  cfg.samplesEnabled(),
		GitHubURL:  "https://github.com/quantumx-apps/go-office",
	})
	if err != nil {
		log.Fatalf("home: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", homeHandler)
	mux.Handle("GET /docs/api", http.HandlerFunc(homeHandler.ServeAPIDocs))
	mux.Handle("GET /docs/api/", http.RedirectHandler("/docs/api", http.StatusPermanentRedirect))
	mux.Handle("/", srv.Handler())

	officeBase := strings.TrimSuffix(origin, "/") + strings.TrimSuffix(srv.BasePath(), "/")
	if officeBase == origin {
		officeBase = origin
	}
	log.Printf("go-office listening on %s", cfg.Addr)
	if debug {
		log.Printf("  debug:   enabled")
	}
	log.Printf("  site:    %s/", strings.TrimSuffix(origin, "/"))
	log.Printf("  health:  %s/health", officeBase)
	log.Printf("  api.js:  %s/web-apps/apps/api/documents/api.js", officeBase)
	if cfg.samplesEnabled() {
		log.Printf("  demo:    %s/demo/", officeBase)
		log.Printf("  api:     %s/demo/config", strings.TrimSuffix(origin, "/")+strings.TrimSuffix(cfg.APIBase, "/"))
		log.Printf("  samples: %s", samplesPath)
	}
	if cfg.JWTSecret != "" {
		log.Printf("  jwt:     enabled (OFFICE_JWT_SECRET)")
	}
	log.Printf("  assets:  %s", cfg.AssetDir)

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: mux}
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("shutting down…")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}()

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

var _ office.Storage = (*localStorage)(nil)
