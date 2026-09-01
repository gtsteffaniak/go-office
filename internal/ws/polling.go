package ws

import (
	"net/http"
	"strings"
)

// IsCoauthoringPollingCheck reports whether r is a routine coauthoring long-poll GET.
// These requests block waiting for packets and dominate debug logs when omitted.
func IsCoauthoringPollingCheck(r *http.Request) bool {
	if r == nil || r.Method != http.MethodGet {
		return false
	}
	if r.URL.Query().Get("transport") != "polling" {
		return false
	}
	_, ok := Match(strings.Trim(r.URL.Path, "/"))
	return ok
}
