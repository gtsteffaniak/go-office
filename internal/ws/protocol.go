package ws

import (
	"encoding/json"
	"strings"
)

// handshakeOK is reported to sdkjs as a successful community handshake.
const handshakeOK = 3

// rightsEdit matches ONLYOFFICE RIGHTS.Edit.
const rightsEdit = 1

func socketMessage(payload any) (string, error) {
	raw, err := json.Marshal([]any{"message", payload})
	if err != nil {
		return "", err
	}
	return "42" + string(raw), nil
}

func serverInfoPacket(build BuildInfo) string {
	pkt, err := socketMessage(map[string]any{
		"type": "license",
		"license": map[string]any{
			"type":               handshakeOK,
			"light":              false,
			"mode":               0,
			"rights":             rightsEdit,
			"buildVersion":       build.BuildVersion,
			"buildNumber":        build.BuildNumber,
			"protectionSupport":  true,
			"isAnonymousSupport": true,
			"liveViewerSupport":  false,
			"branding":           false,
			"customization":      true,
			"advancedApi":        true,
		},
	})
	if err != nil {
		return `42["message",{"type":"license","license":{"type":3,"rights":1,"buildVersion":"0.0.0","buildNumber":0}}]`
	}
	return pkt
}

func joinPackets(packets []string) string {
	return strings.Join(packets, "\x1e")
}

func parsePostPackets(body string) []string {
	if body == "" {
		return nil
	}
	return strings.Split(body, "\x1e")
}

func connectAuthData(packet string) []byte {
	if !strings.HasPrefix(packet, "40") || len(packet) <= 2 {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(packet[2:]), &envelope); err != nil {
		return nil
	}
	return envelope.Data
}

func isAuthSocketPacket(packet string) bool {
	if !strings.HasPrefix(packet, "42") {
		return false
	}
	var parts []json.RawMessage
	if err := json.Unmarshal([]byte(packet[2:]), &parts); err != nil || len(parts) < 2 {
		return false
	}
	var event string
	if err := json.Unmarshal(parts[0], &event); err != nil || event != "message" {
		return false
	}
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(parts[1], &msg); err != nil {
		return false
	}
	return msg.Type == "auth"
}
