package ws

import (
	"encoding/json"
	"strings"
)

// handshakeOK is the ONLYOFFICE/Euro-Office community license type reported
// to sdkjs during coauthoring connect. This is editor protocol metadata, not
// a user-facing EULA or click-through license acceptance step.
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

// Coauthoring close codes (ONLYOFFICE c_oCloseCode). Sent to sdkjs as a `close` message so
// the editor reports a session error instead of waiting for a document that will never open.
const (
	closeCodeJWTMissing = 4005
	closeCodeJWTError   = 4006
)

// closePacket builds a `close` message. sdkjs maps the code onto an editor error
// (c_oCloseCode.jwtError -> Asc.c_oAscError.ID.VKeyEncrypt), so the user sees a refusal
// rather than a silent hang.
func closePacket(code int) string {
	pkt, err := socketMessage(map[string]any{
		"type": "close",
		"data": map[string]any{"code": code},
	})
	if err != nil {
		return ""
	}
	return pkt
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
