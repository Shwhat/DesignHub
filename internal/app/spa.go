package app

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spa serves the built frontend. Unknown paths outside /api return the app
// shell so the browser router can handle them.
func (a *App) spa() http.Handler {
	if a.Web == nil {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.FS(a.Web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, errNotFound)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(a.Web, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					// Vite puts a content hash in asset file names.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		b, err := fs.ReadFile(a.Web, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	})
}
