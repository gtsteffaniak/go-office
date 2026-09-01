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

func messageInt(msg map[string]any, key string, unset int) int {
	raw, ok := msg[key]
	if !ok || raw == nil {
		return unset
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return unset
		}
		return int(n)
	default:
		return unset
	}
}

func lockBlocks(msg map[string]any) []any {
	raw, ok := msg["block"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		return v
	default:
		return []any{v}
	}
}

func lockKey(block any) string {
	switch v := block.(type) {
	case string:
		if v != "" {
			return v
		}
	case map[string]any:
		if g, ok := v["guid"].(string); ok && g != "" {
			return g
		}
	}
	return "lock"
}

func packetTypes(packets []string) []string {
	out := make([]string, 0, len(packets))
	for _, p := range packets {
		if msg, ok := parseSocketMessage(p); ok {
			if t := messageType(msg); t != "" {
				out = append(out, t)
				continue
			}
		}
		if len(p) >= 2 && p[0] == '4' && p[1] == '0' {
			out = append(out, "namespace")
			continue
		}
		if p == "6" {
			out = append(out, "noop")
			continue
		}
		out = append(out, "other")
	}
	return out
}
