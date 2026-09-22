package office

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/quantumx-apps/go-office/pkg/callback"
)

// handleDownloadFile serves the original document bytes for PDF preview and similar
// flows. Euro-Office POSTs to /downloadfile/{documentKey} at the site root with a
// JSON body {url, token} and Range: bytes=0-N for extended-PDF sniffing.
func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key, ok := parseDownloadFileKey(r.URL.Path, s.opts.BasePath)
	if !ok {
		http.NotFound(w, r)
		return
	}
	doc, ok := s.sessions.Lookup(key)
	if !ok || (doc.Path == "" && doc.URL == "") {
		http.NotFound(w, r)
		return
	}
	docURL := doc.URL
	if docURL == "" {
		docURL = doc.Path
	}
	bodyURL, bodyToken := parseDownloadFileBody(r)
	if bodyURL != "" {
		docURL = bodyURL
	}
	if len(s.opts.JWTSecret) > 0 && bodyToken != "" {
		claims, err := callback.VerifySignature(s.opts.JWTSecret, bodyToken)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if bodyURL != "" {
			if u, ok := claims["url"].(string); ok && strings.TrimSpace(u) != "" && strings.TrimSpace(u) != bodyURL {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
		}
	}
	if err := s.serveDocumentBytes(r.Context(), w, r, docURL); err != nil {
		if s.opts.Debug && s.opts.Logger != nil {
			s.opts.Logger.Warn("downloadfile failed", "key", key, "url", docURL, "err", err)
		}
		http.Error(w, "download failed", http.StatusBadGateway)
		return
	}
	if s.opts.Debug && s.opts.Logger != nil {
		s.opts.Logger.Debug("downloadfile ok", "key", key, "url", docURL)
	}
}

func parseDownloadFileKey(urlPath, basePath string) (string, bool) {
	urlPath = strings.TrimPrefix(urlPath, "/")
	candidates := []string{"downloadfile/"}
	if base := strings.Trim(strings.TrimSpace(basePath), "/"); base != "" {
		candidates = append(candidates, base+"/downloadfile/")
	}
	for _, prefix := range candidates {
		if strings.HasPrefix(urlPath, prefix) {
			key := strings.TrimPrefix(urlPath, prefix)
			key = strings.Trim(key, "/")
			if key != "" && !strings.Contains(key, "/") {
				return key, true
			}
		}
	}
	return "", false
}

func parseDownloadFileBody(r *http.Request) (url string, token string) {
	if r.Body == nil {
		return "", ""
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		return "", ""
	}
	var payload struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return "", ""
	}
	return strings.TrimSpace(payload.URL), strings.TrimSpace(payload.Token)
}

func (s *Server) serveDocumentBytes(ctx context.Context, w http.ResponseWriter, r *http.Request, rawURL string) error {
	data, contentType, err := s.fetchDocumentBytes(ctx, rawURL)
	if err != nil {
		return err
	}
	if contentType == "" {
		contentType = mimeByURL(rawURL)
	}

	if rangeHdr := r.Header.Get("Range"); rangeHdr != "" {
		start, end, ok := parseByteRange(rangeHdr, len(data))
		if ok {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			_, err = w.Write(data[start : end+1])
			return err
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	return err
}

func (s *Server) fetchDocumentBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("upstream %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func parseByteRange(hdr string, size int) (start, end int, ok bool) {
	if !strings.HasPrefix(hdr, "bytes=") || size <= 0 {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(hdr, "bytes=")
	if i := strings.Index(spec, ","); i >= 0 {
		spec = spec[:i]
	}
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	var err error
	if parts[0] == "" {
		suffix, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil || suffix <= 0 {
			return 0, 0, false
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true
	}
	start, err = strconv.Atoi(parts[0])
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	if parts[1] == "" {
		end = size - 1
	} else {
		end, err = strconv.Atoi(parts[1])
		if err != nil || end < start {
			return 0, 0, false
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true
}

func mimeByURL(rawURL string) string {
	switch strings.ToLower(path.Ext(rawURL)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}
