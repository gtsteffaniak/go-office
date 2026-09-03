package office

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/netutil"
)

const defaultThumbnailWidth = 200

// RunConverter executes a conversion request and returns the public file URL.
func (s *Server) RunConverter(ctx context.Context, origin string, req ConverterRequest) (ConverterResponse, error) {
	conv, err := s.converter()
	if err != nil {
		return ConverterResponse{}, err
	}

	cacheName := ConvCacheDirName(req.Key, req.OutputType)
	outName := ConvOutputBasename(req.OutputType, req.Thumbnail)
	fileType := ConvOutputFileType(req.OutputType, req.Thumbnail)
	convDir := filepath.Join(s.cacheDir(), cacheName)
	outPath := filepath.Join(convDir, outName)

	if st, statErr := os.Stat(outPath); statErr == nil && st.Size() > 0 {
		return ConverterResponse{
			EndConvert: true,
			FileType:   fileType,
			FileURL:    ConvFileURL(origin, s.opts.BasePath, cacheName, outName, req.Title),
			Percent:    100,
		}, nil
	}

	if err = os.MkdirAll(convDir, 0o755); err != nil {
		return ConverterResponse{}, err
	}

	tmp, err := os.CreateTemp(convDir, "src-*."+req.FileType)
	if err != nil {
		return ConverterResponse{}, err
	}
	srcPath := tmp.Name()
	defer os.Remove(srcPath)

	if err := downloadConverterSource(ctx, req.URL, tmp); err != nil {
		tmp.Close()
		return ConverterResponse{}, fmt.Errorf("converter: download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return ConverterResponse{}, err
	}

	thumb := thumbnailToConvert(req.Thumbnail)
	if err := conv.ConvertFile(ctx, convert.ConvertRequest{
		SourcePath: srcPath,
		DestPath:   outPath,
		FileType:   req.FileType,
		OutputType: req.OutputType,
		Thumbnail:  thumb,
	}); err != nil {
		return ConverterResponse{Error: -4}, err
	}

	return ConverterResponse{
		EndConvert: true,
		FileType:   fileType,
		FileURL:    ConvFileURL(origin, s.opts.BasePath, cacheName, outName, req.Title),
		Percent:    100,
	}, nil
}

func thumbnailToConvert(t *ConverterThumbnail) *convert.Thumbnail {
	if t == nil {
		return &convert.Thumbnail{Width: defaultThumbnailWidth, Height: defaultThumbnailWidth, Aspect: 2, First: true}
	}
	thumb := &convert.Thumbnail{
		Width:  t.Width,
		Height: t.Height,
		Aspect: t.Aspect,
		First:  t.First,
	}
	if thumb.Width <= 0 {
		thumb.Width = defaultThumbnailWidth
	}
	if thumb.Height <= 0 {
		thumb.Height = defaultThumbnailWidth
	}
	if thumb.Aspect < 0 {
		thumb.Aspect = 2
	}
	return thumb
}

func downloadConverterSource(ctx context.Context, rawURL string, dest *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}
	_, err = io.Copy(dest, io.LimitReader(resp.Body, 128<<20))
	return err
}

// VerifyConverterJWT checks Authorization Bearer or body token when JWT secret is set.
func VerifyConverterJWT(secret []byte, authHeader string, body []byte, req ConverterRequest) error {
	if len(secret) == 0 {
		return nil
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		token = req.Token
	}
	if token == "" {
		return fmt.Errorf("converter: missing jwt")
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("converter: unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return fmt.Errorf("converter: invalid jwt: %w", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("converter: invalid claims")
	}
	if v, ok := claimString(claims, "key", "Key"); ok && v != req.Key {
		return fmt.Errorf("converter: jwt key mismatch")
	}
	if v, ok := claimString(claims, "url", "URL"); ok && v != req.URL {
		return fmt.Errorf("converter: jwt url mismatch")
	}
	return nil
}

func claimString(claims jwt.MapClaims, keys ...string) (string, bool) {
	for _, k := range keys {
		if raw, ok := claims[k]; ok {
			if s, ok := raw.(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}

func (s *Server) handleConverter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req, err := ParseConverterRequest(body)
	if err != nil {
		writeConverterError(w, r, -4, err.Error())
		return
	}
	if jwtErr := VerifyConverterJWT(s.opts.JWTSecret, r.Header.Get("Authorization"), body, req); jwtErr != nil {
		if s.opts.Debug && s.opts.Logger != nil {
			s.opts.Logger.Debug("converter jwt", "err", jwtErr)
		}
		writeConverterError(w, r, -20, "token")
		return
	}
	if req.Async {
		writeConverterError(w, r, -4, "async not supported")
		return
	}

	origin := strings.TrimSuffix(netutil.RequestOrigin(r), "/")
	if o := strings.TrimSpace(s.opts.PublicOrigin); o != "" {
		origin = strings.TrimSuffix(o, "/")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	res, err := s.RunConverter(ctx, origin, req)
	if err != nil {
		if s.opts.Logger != nil {
			s.opts.Logger.Error("converter failed", "key", req.Key, "err", err)
		}
		if res.Error == nil {
			res.Error = -4
		}
		writeConverterResponse(w, r, res)
		return
	}
	writeConverterResponse(w, r, res)
}

func writeConverterResponse(w http.ResponseWriter, r *http.Request, res ConverterResponse) {
	if wantsJSONConverterResponse(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	errVal := ""
	if res.Error != nil {
		errVal = fmt.Sprint(res.Error)
	}
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?><FileResult><EndConvert>%t</EndConvert><FileType>%s</FileType><FileUrl>%s</FileUrl><Percent>%d</Percent><Error>%s</Error></FileResult>`,
		res.EndConvert, xmlEscape(res.FileType), xmlEscape(res.FileURL), res.Percent, xmlEscape(errVal))
}

func writeConverterError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	writeConverterResponse(w, r, ConverterResponse{EndConvert: false, Error: code, Percent: 0})
	if s := w.Header().Get("Content-Type"); s == "" && wantsJSONConverterResponse(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	_ = msg
}

func wantsJSONConverterResponse(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	ct := r.Header.Get("Content-Type")
	return strings.Contains(ct, "application/json")
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
