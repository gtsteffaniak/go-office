package ws

import (
	"encoding/json"
	"strings"
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
	Type  string   `json:"type"`
	DocID string   `json:"docid"`
	User  authUser `json:"user"`
	Mode  string   `json:"mode"`
	Open  *openCmd `json:"openCmd"`
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
