package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const defaultSessionID = "go-office"

type session struct {
	docKey   string
	build    BuildInfo
	basePath string

	mu sync.Mutex

	outbox  []string
	waitCh  chan struct{}
	waitGen uint64

	namespaceAck   bool
	infoSent       bool
	authSent       bool
	openStarted    bool
	documentOpened bool

	sessionID string
	indexUser int
	userID    string // participant id: original user id + indexUser (sdkjs _userId)

	configEpochAtCreate uint64
}

var coauthoringSessions = sessionRegistry{sessions: make(map[string]*session)}

type sessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

func sessionKey(sid, docKey string) string {
	if sid == "" {
		sid = defaultSessionID
	}
	return sid + "\x00" + docKey
}

func getSession(sid, docKey string, build BuildInfo, basePath string) *session {
	return coauthoringSessions.get(sid, docKey, build, basePath)
}

func (r *sessionRegistry) get(sid, docKey string, build BuildInfo, basePath string) *session {
	key := sessionKey(sid, docKey)

	r.mu.RLock()
	s, ok := r.sessions[key]
	r.mu.RUnlock()
	if ok {
		s.applyBuildBase(build, basePath)
		return s
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok = r.sessions[key]; ok {
		s.applyBuildBase(build, basePath)
		return s
	}
	s = &session{
		docKey:              docKey,
		build:               build,
		basePath:            basePath,
		indexUser:           1,
		configEpochAtCreate: documentConfigEpoch(docKey),
	}
	r.sessions[key] = s
	return s
}

func (s *session) applyBuildBase(build BuildInfo, basePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.build.Release == "" && build.Release != "" {
		s.build = build
	}
	if s.basePath == "" && basePath != "" {
		s.basePath = basePath
	}
}

// ResetSessionsForTest clears in-memory coauthoring sessions (tests only).
func ResetSessionsForTest() {
	resetDocumentConfigEpochs()
	ClearAllSessions()
}

// ClearAllSessions drops all in-memory coauthoring sessions.
func ClearAllSessions() {
	coauthoringSessions.clear()
}

// ClearDocumentSession drops the default demo coauthoring session for a document key.
// Call when a new editor page loads so polling reconnect is not confused with reload.
func ClearDocumentSession(docKey string) {
	if docKey == "" {
		return
	}
	bumpDocumentConfigEpoch(docKey)
	coauthoringSessions.delete(sessionKey(defaultSessionID, docKey))
}

func (r *sessionRegistry) delete(key string) {
	r.mu.Lock()
	delete(r.sessions, key)
	r.mu.Unlock()
}

func (r *sessionRegistry) clear() {
	r.mu.Lock()
	r.sessions = make(map[string]*session)
	r.mu.Unlock()
}

func forEachSession(docKey string, fn func(*session)) {
	coauthoringSessions.mu.RLock()
	defer coauthoringSessions.mu.RUnlock()
	for _, s := range coauthoringSessions.sessions {
		if s.docKey == docKey {
			fn(s)
		}
	}
}

func (s *session) enqueue(packets ...string) {
	s.mu.Lock()
	s.outbox = append(s.outbox, packets...)
	if s.waitCh != nil {
		close(s.waitCh)
		s.waitCh = nil
	}
	s.mu.Unlock()
}

func (s *session) drain() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.outbox) == 0 {
		return nil
	}
	out := s.outbox
	s.outbox = nil
	return out
}

func (s *session) waitForPackets(ctx context.Context, hold time.Duration) []string {
	if packets := s.drain(); len(packets) > 0 {
		return packets
	}
	if hold <= 0 {
		return nil
	}

	s.mu.Lock()
	if len(s.outbox) > 0 {
		out := s.outbox
		s.outbox = nil
		s.mu.Unlock()
		return out
	}
	if s.waitCh == nil {
		s.waitCh = make(chan struct{})
	}
	ch := s.waitCh
	gen := s.waitGen
	s.mu.Unlock()

	timer := time.NewTimer(hold)
	defer timer.Stop()

	select {
	case <-ch:
		s.mu.Lock()
		stale := s.waitGen != gen
		s.mu.Unlock()
		if stale {
			return nil
		}
		return s.drain()
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return nil
	}
}

func (s *session) isReconnectAuth(req authRequest) bool {
	if !s.documentOpened || s.sessionID == "" {
		return false
	}
	if req.SessionID != "" {
		return req.SessionID == s.sessionID
	}
	// Engine.IO packet 40 on polling reconnect often omits sessionId; the editor
	// still holds the open document and resends sessionId on the follow-up auth.
	return true
}

func (s *session) needsDocumentOpen(req authRequest) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncConfigEpochLocked()
	if req.Open == nil {
		return false
	}
	if s.documentOpened || s.openStarted {
		return false
	}
	return !s.isReconnectAuth(req)
}

func (s *session) syncConfigEpochLocked() {
	current := documentConfigEpoch(s.docKey)
	if current > s.configEpochAtCreate {
		s.documentOpened = false
		s.openStarted = false
		s.configEpochAtCreate = current
	}
}

func (s *session) shouldLogReconnect(req authRequest) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.documentOpened && s.isReconnectAuth(req)
}

func (s *session) onConnect(authData []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	reconnect := false
	if len(authData) > 0 {
		if req, ok := parseAuthPayload(authData); ok {
			reconnect = s.isReconnectAuth(req)
		}
	}

	// Engine.IO packet 40 is a new transport session. The demo client always
	// reuses sid=go-office, so a CSV reload would otherwise skip auth and hang.
	// When the client reconnects with the same coauthoring sessionId, resend auth
	// but do not re-open the document — the editor still holds in-memory state.
	s.authSent = false
	if !reconnect {
		s.documentOpened = false
		s.openStarted = false
	}
	s.waitGen++
	if s.waitCh != nil {
		close(s.waitCh)
		s.waitCh = nil
	}
	s.outbox = append(s.outbox, `40{"sid":"`+defaultSessionID+`"}`)
	s.namespaceAck = true
	s.outbox = append(s.outbox, serverInfoPacket(s.build))
	s.infoSent = true
	if len(authData) > 0 {
		if req, ok := parseAuthPayload(authData); ok {
			s.queueAuthLocked(req)
		}
	}
	s.signalWaitersLocked()
}

func (s *session) onAuth(req authRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queueAuthLocked(req)
	s.signalWaitersLocked()
}

func (s *session) queueAuthLocked(req authRequest) {
	userID := req.User.ID
	if userID == "" {
		userID = "user"
	}
	s.userID = fmt.Sprintf("%s%d", userID, s.indexUser)
	if !s.authSent {
		s.authSent = true
		if s.sessionID == "" {
			s.sessionID = newSessionID()
		}
		s.outbox = append(s.outbox, authResponsePackets(s.build, s.sessionID, s.indexUser, req)...)
	}
}

func (s *session) participantID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.userID != "" {
		return s.userID
	}
	return fmt.Sprintf("user%d", s.indexUser)
}

func (s *session) startOpen(opener DocumentOpener, req authRequest, origin string) {
	if opener == nil || req.Open == nil {
		return
	}

	s.mu.Lock()
	if s.documentOpened || s.openStarted || s.isReconnectAuth(req) {
		s.mu.Unlock()
		return
	}
	s.openStarted = true
	docKey := s.docKey
	basePath := s.basePath
	open := *req.Open
	s.mu.Unlock()

	docOpenInflight.Add(1)
	go func() {
		defer docOpenInflight.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		packets, err := opener.Open(ctx, origin, basePath, docKey, open)
		if err != nil {
			s.mu.Lock()
			s.openStarted = false
			s.mu.Unlock()
			if pkt, perr := documentOpenPacket(open.Command, "error", err.Error()); perr == nil {
				s.enqueue(pkt)
			}
			return
		}
		if len(packets) > 0 {
			s.enqueue(packets...)
			s.mu.Lock()
			s.documentOpened = true
			s.mu.Unlock()
		}
		s.mu.Lock()
		s.openStarted = false
		s.mu.Unlock()
	}()
}

func (s *session) signalWaitersLocked() {
	if s.waitCh != nil {
		close(s.waitCh)
		s.waitCh = nil
	}
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return defaultSessionID
	}
	return hex.EncodeToString(b[:])
}

func authResponsePackets(build BuildInfo, sessionID string, indexUser int, req authRequest) []string {
	now := time.Now().UnixMilli()
	userID := req.User.ID
	if userID == "" {
		userID = "user"
	}
	username := req.User.Username
	if username == "" {
		username = userID
	}
	participants := []any{
		map[string]any{
			"id":                 fmt.Sprintf("%s%d", userID, indexUser),
			"idOriginal":         userID,
			"username":           username,
			"indexUser":          indexUser,
			"view":               req.Mode == "view",
			"connectionId":       sessionID,
			"isCloseCoAuthoring": false,
		},
	}

	changes, _ := socketMessage(map[string]any{
		"type":    "authChanges",
		"changes": []any{},
	})
	auth, _ := socketMessage(map[string]any{
		"type":               "auth",
		"result":             1,
		"sessionId":          sessionID,
		"sessionTimeConnect": now,
		"participants":       participants,
		"locks":              map[string]any{},
		"indexUser":          indexUser,
		"hasForgotten":       false,
		"buildVersion":       build.BuildVersion,
		"buildNumber":        build.BuildNumber,
		"licenseType":        handshakeOK,
		"settings": map[string]any{
			"binaryChanges":           false,
			"websocketMaxPayloadSize": 1572864,
			"maxChangesSize":          157286400,
			"reconnection": map[string]any{
				"attempts": 50,
				"delay":    2000,
			},
		},
		"openedAt": now,
	})
	return []string{changes, auth}
}
