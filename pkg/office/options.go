package office

import (
	"log/slog"
	"strings"
)

// DefaultBasePath is the default URL prefix for the embedded document server.
// Matches ONLYOFFICE Document Server (assets and coauthoring at the site root).
const DefaultBasePath = "/"

// Options configures a document server instance.
type Options struct {
	// AssetDir is the root directory containing web-apps/, sdkjs/, and optionally
	// converter binaries. Populated from a Euro-Office Document Server build.
	AssetDir string

	// BasePath is the full URL prefix where this server is mounted.
	// Defaults to DefaultBasePath (/). Use JoinBasePath to combine
	// an application subpath (e.g. FileBrowser http.baseURL) with the office mount.
	// Must not include a trailing slash.
	BasePath string

	// JWTSecret signs and verifies editor configuration tokens when non-empty.
	JWTSecret []byte

	// ProtocolVersion is reported in coauthoring URLs (e.g. "9.0.4-abc123").
	// Must match the pinned Euro-Office asset build.
	ProtocolVersion string

	// ConvertLimit caps concurrent x2t subprocess conversions (default 1).
	ConvertLimit int

	// Debug enables verbose request and protocol logging.
	Debug bool

	Logger *slog.Logger
}

// JoinBasePath combines an application base URL path with the office mount path.
// appBase is typically the host app's configured subpath (e.g. "/myapp/" or "/").
// mount defaults to DefaultBasePath when empty.
//
//	JoinBasePath("/", "")           -> "/"
//	JoinBasePath("/myapp/", "")     -> "/myapp/"
//	JoinBasePath("/myapp", "office") -> "/myapp/office"
func JoinBasePath(appBase, mount string) string {
	if mount == "" {
		mount = DefaultBasePath
	}
	app := strings.Trim(appBase, "/")
	mount = strings.Trim(mount, "/")
	if app == "" {
		return "/" + mount
	}
	return "/" + app + "/" + mount
}

func (o *Options) normalize() {
	o.BasePath = normalizePath(o.BasePath)
	if o.ProtocolVersion == "" {
		o.ProtocolVersion = "0.0.0-dev"
	}
	if o.ConvertLimit <= 0 {
		o.ConvertLimit = 1
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Debug {
		o.Logger.Debug("go-office debug logging enabled")
	}
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return "/"
	}
	p = strings.TrimSuffix(p, "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
