package ws

// OpenCmd is the document open command from a coauthoring auth payload.
type OpenCmd = openCmd

// DocumentOpenOKPacket builds a successful documentOpen socket packet for tests.
func DocumentOpenOKPacket(cmdType string, files map[string]string) (string, error) {
	return documentOpenPacket(cmdType, "ok", files)
}
