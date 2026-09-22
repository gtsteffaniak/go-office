package callback

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// SignBody wraps payload JSON as a JWT string for JWT_IN_BODY callbacks.
func SignBody(secret []byte, body []byte) (string, error) {
	if len(secret) == 0 {
		return "", nil
	}
	var claims jwt.MapClaims
	if err := json.Unmarshal(body, &claims); err != nil {
		return "", err
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(secret)
}

// VerifySignature validates an HS256 JWT and returns its claims without requiring
// callback-shaped fields (e.g. editor config tokens with document.* claims).
func VerifySignature(secret []byte, token string) (jwt.MapClaims, error) {
	if len(secret) == 0 || token == "" {
		return nil, fmt.Errorf("callback: missing token")
	}
	token = strings.TrimSpace(token)
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("callback: unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("callback: invalid token: %w", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("callback: invalid claims")
	}
	return claims, nil
}

// VerifyBody validates a JWT token and returns the callback payload.
func VerifyBody(secret []byte, token string) (Payload, error) {
	claims, err := VerifySignature(secret, token)
	if err != nil {
		return Payload{}, err
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return Payload{}, err
	}
	return Parse(raw)
}

// ParseRequest reads a callback body that may be raw JSON or {"token":"..."}.
func ParseRequest(body []byte, secret []byte) (Payload, error) {
	if len(body) == 0 {
		return Payload{}, fmt.Errorf("callback: empty body")
	}
	var wrapped struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Token != "" {
		if len(secret) > 0 {
			return VerifyBody(secret, wrapped.Token)
		}
		parsed, err := jwt.Parse(wrapped.Token, func(t *jwt.Token) (any, error) {
			return secret, nil
		})
		if err == nil && parsed.Valid {
			if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
				raw, _ := json.Marshal(claims)
				return Parse(raw)
			}
		}
	}
	p := Payload{}
	if err := json.Unmarshal(body, &p); err != nil {
		return Payload{}, err
	}
	if p.Key == "" {
		return Payload{}, fmt.Errorf("callback: missing key")
	}
	return p, nil
}
