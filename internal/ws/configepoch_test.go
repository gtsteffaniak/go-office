package ws

import (
	"testing"
)

func TestDocumentConfigEpochForcesReopenWithoutSessionDelete(t *testing.T) {
	ResetSessionsForTest()
	docKey := "epoch-key"
	sess := getSession(defaultSessionID, docKey, ParseBuild("9.3.4"), "")

	sess.mu.Lock()
	sess.documentOpened = true
	sess.sessionID = "coauth-session"
	sess.mu.Unlock()

	bumpDocumentConfigEpoch(docKey)
	if documentConfigEpoch(docKey) <= sess.configEpochAtCreate {
		t.Fatalf("epoch = %d, want > %d", documentConfigEpoch(docKey), sess.configEpochAtCreate)
	}

	req := authRequest{
		Open: &openCmd{Command: "open", Format: "docx", URL: "http://localhost/f.docx"},
	}
	if !sess.needsDocumentOpen(req) {
		t.Fatal("new config epoch should force document reopen")
	}
}

func TestDocumentConfigEpochUnchangedSkipsReopenOnReconnect(t *testing.T) {
	ResetSessionsForTest()
	docKey := "epoch-reconnect"
	sess := getSession(defaultSessionID, docKey, ParseBuild("9.3.4"), "")

	sess.mu.Lock()
	sess.documentOpened = true
	sess.sessionID = "coauth-session"
	sess.configEpochAtCreate = documentConfigEpoch(docKey)
	sess.mu.Unlock()

	req := authRequest{
		SessionID: "coauth-session",
		Open:      &openCmd{Command: "open", Format: "docx", URL: "http://localhost/f.docx"},
	}
	if sess.needsDocumentOpen(req) {
		t.Fatal("unchanged epoch with matching sessionId should not reopen")
	}
}
