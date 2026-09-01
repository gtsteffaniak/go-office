package office

import "net/http"

// RequestOrigin returns the browser-visible origin (scheme + host) for an HTTP
// request, honoring X-Forwarded-Proto and X-Forwarded-Host when present.
func RequestOrigin(r *http.Request) string {
	if r == nil {
		return "http://localhost"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
		return proto + "://" + host
	}
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}
