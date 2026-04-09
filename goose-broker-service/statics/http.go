package statics

import (
	"embed"
	"net/http"
	"net/url"
)

//go:embed www/*
var www embed.FS

func ServeStatics(staticsDir string) http.HandlerFunc {
	if staticsDir == "" {
		return addPrefix("www", http.FileServer(http.FS(www)))
	}
	return http.FileServer(http.Dir(staticsDir)).ServeHTTP
}

func addPrefix(prefix string, h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := prefix + r.URL.Path
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p
		h.ServeHTTP(w, r2)
	}
}
