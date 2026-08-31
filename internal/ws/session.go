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
	docKey  string
	build   BuildInfo
	basePath string

	mu sync.Mutex

	outbox []string
	waitCh chan struct{}

	namespaceAck bool
	infoSent     bool
	authSent     bool
	openStarted  bool

	sessionID string
	indexUser int
}

var sessions sync.Map // sessionKey -> *session

func sessionKey(sid, docKey string) string {
	if sid == "" {
		sid = defaultSessionID
	}
	return sid + "\x00" + docKey
}

func getSession(sid, docKey string, build BuildInfo, basePath string) *session {
	key := sessionKey(sid, docKey)
	if v, ok := sessions.Load(key); ok {
		s := v.(*session)
		if s.build.Release == "" && build.Release != "" {
			s.build = build
		}
		if s.basePath == "" && basePath != "" {
			s.basePath = basePath
		}
		return s
	}
	s := &session{docKey: docKey, build: build, basePath: basePath, indexUser: 1}
	actual, _ := sessions.LoadOrStore(key, s)
	return actual.(*session)
}

// ResetSessionsForTest clears in-memory coauthoring sessions (tests only).
func ResetSessionsForTest() {
	sessions = sync.Map{}
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
	s.mu.Unlock()

	timer := time.NewTimer(hold)
	defer timer.Stop()

	select {
	case <-ch:
		return s.drain()
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return nil
	}
}

func (s *session) onConnect(authData []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.namespaceAck {
		s.namespaceAck = true
		s.outbox = append(s.outbox, `40{"sid":"`+defaultSessionID+`"}`)
	}
	if !s.infoSent {
		s.infoSent = true
		s.outbox = append(s.outbox, serverInfoPacket(s.build))
	}
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
	if !s.authSent {
		s.authSent = true
		if s.sessionID == "" {
			s.sessionID = newSessionID()
		}
		s.outbox = append(s.outbox, authResponsePackets(s.build, s.sessionID, s.indexUser, req)...)
	}
}

func (s *session) startOpen(opener *Opener, req authRequest, origin string) {
	if opener == nil || req.Open == nil {
		return
	}

	s.mu.Lock()
	if s.openStarted {
		s.mu.Unlock()
		return
	}
	s.openStarted = true
	docKey := s.docKey
	basePath := s.basePath
	open := *req.Open
	s.mu.Unlock()

	go func() {
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
		}
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
			"reconnection": map[string]any{
				"attempts": 50,
				"delay":    2000,
			},
		},
		"openedAt": now,
	})
	return []string{changes, auth}
}
