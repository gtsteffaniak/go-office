package office

import "net/http"

func (s *Server) httpClient() *http.Client {
	if s != nil && s.opts.HTTPClient != nil {
		return s.opts.HTTPClient
	}
	return http.DefaultClient
}
