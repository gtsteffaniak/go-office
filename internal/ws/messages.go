package ws

import "encoding/json"

// parseSocketMessage decodes a 42["message", payload] packet into payload.
func parseSocketMessage(packet string) (map[string]any, bool) {
	if len(packet) < 2 || packet[0] != '4' || packet[1] != '2' {
		return nil, false
	}
	var parts []json.RawMessage
	if err := json.Unmarshal([]byte(packet[2:]), &parts); err != nil || len(parts) < 2 {
		return nil, false
	}
	var event string
	if err := json.Unmarshal(parts[0], &event); err != nil || event != "message" {
		return nil, false
	}
	var msg map[string]any
	if err := json.Unmarshal(parts[1], &msg); err != nil {
		return nil, false
	}
	return msg, true
}

func messageType(msg map[string]any) string {
	t, _ := msg["type"].(string)
	return t
}

func messageBool(msg map[string]any, key string) bool {
	v, ok := msg[key].(bool)
	return ok && v
}
