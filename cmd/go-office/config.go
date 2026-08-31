package main

import (
	"flag"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	office "github.com/quantumx-apps/go-office/pkg/office"
	"github.com/quantumx-apps/go-office/internal/debuglog"
	"github.com/quantumx-apps/go-office/internal/demo"
	"github.com/quantumx-apps/go-office/internal/home"
)

// runConfig holds server startup options from flags and OFFICE_* environment variables.
type runConfig struct {
	AssetDir       string
	DataDir        string
	SamplesDir     string
	Addr           string
	BasePath       string
	APIBase        string
	JWTSecret      string
	Version        string
	PublicOrigin   string
	DisableSamples bool
	Debug          bool
	PollHold       *time.Duration
	SaveDelay      *time.Duration
	ConvertLimit   int
}

func parseRunConfig() runConfig {
	cfg := runConfig{
		AssetDir:       office.AssetDirFromEnv(""),
		DataDir:        envFirst("OFFICE_DATA_DIR", "."),
		SamplesDir:     envFirst("OFFICE_SAMPLES_DIR", demo.DefaultSamplesDir),
		Addr:           envFirst("OFFICE_ADDR", ":8080"),
		BasePath:       envFirst("OFFICE_BASE_PATH", office.DefaultBasePath),
		APIBase:        envFirst("OFFICE_API_BASE", home.DefaultAPIBasePath),
		JWTSecret:      jwtSecretFromEnv(),
		Version:        strings.TrimSpace(os.Getenv("OFFICE_VERSION")),
		PublicOrigin:   strings.TrimSpace(os.Getenv("OFFICE_PUBLIC_ORIGIN")),
		DisableSamples: envBool("OFFICE_DISABLE_SAMPLES"),
		Debug:          debugFromEnv(),
		PollHold:       pollHoldFromEnv(),
		SaveDelay:      saveDelayFromEnv(),
		ConvertLimit:   convertLimitFromEnv(),
	}

	flag.StringVar(&cfg.AssetDir, "assets", cfg.AssetDir, "path to Euro-Office assets (web-apps/, sdkjs/)")
	flag.StringVar(&cfg.DataDir, "data", cfg.DataDir, "document root (contains sample-files/ when samples enabled)")
	flag.StringVar(&cfg.SamplesDir, "samples", cfg.SamplesDir, "sample documents directory relative to -data")
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	flag.StringVar(&cfg.BasePath, "base", cfg.BasePath, "document server URL prefix (default / for ONLYOFFICE compatibility)")
	flag.StringVar(&cfg.APIBase, "api-base", cfg.APIBase, "demo API URL prefix (config, callback, file)")
	flag.StringVar(&cfg.JWTSecret, "jwt", cfg.JWTSecret, "JWT secret for editor config signing (OFFICE_JWT_SECRET)")
	flag.StringVar(&cfg.Version, "version", cfg.Version, "protocol version (default: assets/VERSION)")
	flag.StringVar(&cfg.PublicOrigin, "public", cfg.PublicOrigin, "public origin for document URLs")
	flag.BoolVar(&cfg.DisableSamples, "disable-samples", cfg.DisableSamples, "do not serve the demo UI or sample documents")
	debugFlag := flag.Bool("debug", cfg.Debug, "enable verbose logging (or set OFFICE_DEBUG_LOGGING=1)")
	flag.Parse()

	if *debugFlag {
		cfg.Debug = true
	}
	return cfg
}

func (c runConfig) samplesEnabled() bool {
	return !c.DisableSamples
}

func (c runConfig) publicOrigin() string {
	if c.PublicOrigin != "" {
		return strings.TrimSuffix(c.PublicOrigin, "/")
	}
	host, port, err := net.SplitHostPort(c.Addr)
	if err != nil {
		return "http://localhost"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	if port == "" {
		port = "8080"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func envFirst(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func jwtSecretFromEnv() string {
	return strings.TrimSpace(os.Getenv("OFFICE_JWT_SECRET"))
}

func debugFromEnv() bool {
	return debuglog.EnvEnabled()
}

func pollHoldFromEnv() *time.Duration {
	v := strings.TrimSpace(os.Getenv("OFFICE_POLL_HOLD"))
	if v == "" {
		return nil
	}
	if d, err := time.ParseDuration(v); err == nil {
		return &d
	}
	if sec, err := strconv.Atoi(v); err == nil && sec >= 0 {
		d := time.Duration(sec) * time.Second
		return &d
	}
	return nil
}

func convertLimitFromEnv() int {
	v := strings.TrimSpace(os.Getenv("OFFICE_CONVERT_LIMIT"))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func saveDelayFromEnv() *time.Duration {
	v := strings.TrimSpace(os.Getenv("OFFICE_SAVE_DELAY"))
	if v == "" {
		return nil
	}
	if d, err := time.ParseDuration(v); err == nil && d >= 0 {
		return &d
	}
	return nil
}
