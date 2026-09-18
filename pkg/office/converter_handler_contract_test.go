package office_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/convert"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestHandleConverterMissingJWT(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{
		AssetDir:  t.TempDir(),
		JWTSecret: []byte("secret"),
	})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"async":false,"filetype":"docx","key":"k","outputtype":"pdf","url":"http://example.com/doc.docx"}`
	req := httptest.NewRequest(http.MethodPost, "/converter", strings.NewReader(body))
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	var res office.ConverterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if errorCode(res.Error) != convert.ErrCodeInvalidToken {
		t.Fatalf("error = %v, want %d", res.Error, convert.ErrCodeInvalidToken)
	}
}

func TestHandleConverterMalformedInput(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{AssetDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/converter", strings.NewReader(`{not-json`))
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	var res office.ConverterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if errorCode(res.Error) != -4 {
		t.Fatalf("error = %v, want -4", res.Error)
	}
}

func errorCode(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	default:
		return 0
	}
}
