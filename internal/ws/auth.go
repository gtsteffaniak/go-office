package ws

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type openCmd struct {
	Command string `json:"c"`
	ID      string `json:"id"`
	Format  string `json:"format"`
	URL     string `json:"url"`
	Title   string `json:"title"`
}

type authUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type authRequest struct {
	Type                string   `json:"type"`
	DocID               string   `json:"docid"`
	User                authUser `json:"user"`
	Mode                string   `json:"mode"`
	Open                *openCmd `json:"openCmd"`
	SessionID           string   `json:"sessionId"`
	DocumentCallbackURL string   `json:"documentCallbackUrl"`
	CallbackURL         string   `json:"callbackUrl"`
	Token               string   `json:"token"`
}

// IntegratorCallbackURL returns the editor callback URL from coauthoring auth.
func (r authRequest) IntegratorCallbackURL() string {
	if r.DocumentCallbackURL != "" {
		return r.DocumentCallbackURL
	}
	return r.CallbackURL
}

func verifyAuthJWT(secret []byte, token, docKey string) error {
	if len(secret) == 0 {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("coauthoring: missing jwt")
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("coauthoring: unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return fmt.Errorf("coauthoring: invalid jwt: %w", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("coauthoring: invalid claims")
	}
	if docKey != "" {
		if doc, ok := claims["document"].(map[string]any); ok {
			if key, ok := doc["key"].(string); ok && key != "" && key != docKey {
				return fmt.Errorf("coauthoring: jwt document key mismatch")
			}
		}
	}
	return nil
}

func parseAuthPayload(raw []byte) (authRequest, bool) {
	if len(raw) == 0 {
		return authRequest{}, false
	}
	var req authRequest
	if err := json.Unmarshal(raw, &req); err != nil || req.Type != "auth" {
		return authRequest{}, false
	}
	return req, true
}

func parseAuthPacket(packet string) (authRequest, bool) {
	if strings.HasPrefix(packet, "40") && len(packet) > 2 {
		return parseAuthPayload(connectAuthData(packet))
	}
	if !strings.HasPrefix(packet, "42") {
		return authRequest{}, false
	}
	var parts []json.RawMessage
	if err := json.Unmarshal([]byte(packet[2:]), &parts); err != nil || len(parts) < 2 {
		return authRequest{}, false
	}
	var event string
	if err := json.Unmarshal(parts[0], &event); err != nil || event != "message" {
		return authRequest{}, false
	}
	var req authRequest
	if err := json.Unmarshal(parts[1], &req); err != nil || req.Type != "auth" {
		return authRequest{}, false
	}
	return req, true
}
