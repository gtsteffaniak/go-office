package office

import (
	"log/slog"
	"strings"
)

// DefaultBasePath is the default URL prefix for the embedded document server.
const DefaultBasePath = "/api/office"

// Options configures a document server instance.
type Options struct {
	// AssetDir is the root directory containing web-apps/, sdkjs/, and optionally
	// converter binaries. Populated from a Euro-Office Document Server build.
	AssetDir string

	// BasePath is the full URL prefix where this server is mounted.
	// Defaults to DefaultBasePath (/api/office). Use JoinBasePath to combine
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

	Logger *slog.Logger
}

// JoinBasePath combines an application base URL path with the office mount path.
// appBase is typically the host app's configured subpath (e.g. "/myapp/" or "/").
// mount defaults to DefaultBasePath when empty.
//
//	JoinBasePath("/", "")           -> "/api/office"
//	JoinBasePath("/myapp/", "")     -> "/myapp/api/office"
//	JoinBasePath("/myapp", "api/office") -> "/myapp/api/office"
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
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return DefaultBasePath
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
