package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static
var embedded embed.FS
var staticFS, _ = fs.Sub(embedded, "static")
var assetVersion = digestAssets()

func digestAssets() string {
	h := sha256.New()
	_ = fs.WalkDir(staticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			b, e := fs.ReadFile(staticFS, path)
			if e != nil {
				return e
			}
			_, _ = h.Write([]byte(path))
			_, _ = h.Write(b)
		}
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:16]
}
func asset(path string) string { return "/assets/" + assetVersion + "/" + path }
func assets() http.Handler {
	f := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/assets/" + assetVersion + "/"
		if !strings.HasPrefix(r.URL.Path, prefix) || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.StripPrefix(strings.TrimSuffix(prefix, "/"), f).ServeHTTP(w, r)
	})
}
