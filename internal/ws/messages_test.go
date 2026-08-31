package ws

import "testing"

func TestPacketTypes(t *testing.T) {
	auth, err := socketMessage(map[string]any{"type": "getLock", "locks": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	got := packetTypes([]string{`40{"sid":"go-office"}`, auth, "6"})
	if len(got) != 3 || got[0] != "namespace" || got[1] != "getLock" || got[2] != "noop" {
		t.Fatalf("packetTypes = %v", got)
	}
}
