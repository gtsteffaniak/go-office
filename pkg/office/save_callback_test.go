package office_test

import (
	"context"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/session"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestPersistDocumentCallbackOnlySkipsStoragePathCheck(t *testing.T) {
	store := &recordingStorage{t: t}
	srv, err := office.New(store, office.Options{AssetDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	docKey := "callback-only-key"
	srv.Sessions().UpsertDoc(session.Document{
		Key:         docKey,
		CallbackURL: "http://host/callback",
		FileType:    "xlsx",
		URL:         "http://host/view",
	})

	err = srv.PersistDocument(context.Background(), docKey)
	if err == nil {
		t.Fatal("expected error without Editor.bin")
	}
	if strings.Contains(err.Error(), "unknown document key") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "no storage path") {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedPath != "" {
		t.Fatalf("storage.Save should not run in callback-only mode, got path %q", store.savedPath)
	}
}

func TestPersistDocumentRequiresPathOrCallback(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{AssetDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	srv.Sessions().UpsertDoc(session.Document{
		Key:      "orphan-key",
		FileType: "xlsx",
	})
	err = srv.PersistDocument(context.Background(), "orphan-key")
	if err == nil || !strings.Contains(err.Error(), "no storage path or callback URL") {
		t.Fatalf("PersistDocument() = %v", err)
	}
}
