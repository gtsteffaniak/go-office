package office

import "strings"

// URLPath joins a mount prefix and route element.
func URLPath(base, elem string) string {
	return joinURLPath(base, elem)
}

func joinURLPath(base, elem string) string {
	elem = strings.TrimPrefix(strings.TrimSpace(elem), "/")
	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	if base == "" {
		return "/" + elem
	}
	return base + "/" + elem
}
