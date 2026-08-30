package callback

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Status values from ONLYOFFICE callback handler documentation.
const (
	StatusEditing      = 1
	StatusMustSave     = 2
	StatusSaveError    = 3
	StatusClosedNoEdit = 4
	StatusForceSaved   = 6
	StatusForceError   = 7
)

// Payload is the JSON body POSTed to editorConfig.callbackUrl.
type Payload struct {
	Key    string `json:"key"`
	Status int    `json:"status"`
	URL    string `json:"url"`
}

// Parse reads a callback JSON body.
func Parse(body []byte) (Payload, error) {
	var p Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Payload{}, err
	}
	if p.Key == "" {
		return Payload{}, fmt.Errorf("callback: missing key")
	}
	return p, nil
}

// ShouldPersist reports whether the integrator should download url and save.
func (p Payload) ShouldPersist() bool {
	return p.Status == StatusMustSave || p.Status == StatusForceSaved
}

// ReadBody reads and parses a callback request body.
func ReadBody(r io.Reader) (Payload, error) {
	body, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return Payload{}, err
	}
	return Parse(body)
}

// Response writes the standard ONLYOFFICE callback success JSON.
func WriteOK(w interface{ Write([]byte) (int, error) }) {
	_, _ = w.Write([]byte(`{"error":0}`))
}

// WriteError writes a callback error response.
func WriteError(w interface{ Write([]byte) (int, error) }, code int) {
	_, _ = fmt.Fprintf(w, `{"error":%d}`, code)
}

// TrimToken returns a bearer token from Authorization header.
func TrimToken(auth string) string {
	auth = strings.TrimSpace(auth)
	if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return auth
}
