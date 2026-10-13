package storage

import (
	"net/http"
	"net/url"
	"strings"
)

type managementResponse struct{ http.ResponseWriter }

func (w managementResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w managementResponse) WriteHeader(code int) {
	if value := w.Header().Get("Location"); value != "" {
		u, e := url.Parse(value)
		if e == nil {
			w.Header().Set("Location", strings.Replace(u.RequestURI(), "/storage/v1/", "/api/v1/storage/", 1))
		}
	}
	w.ResponseWriter.WriteHeader(code)
}
