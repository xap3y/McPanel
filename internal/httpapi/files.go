package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"mcpanel/internal/fsutil"
	"mcpanel/internal/mcserver"
)

type fileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

func (a *API) serverDirOr404(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return "", false
	}
	return mcserver.ServerDir(a.MCRoot, name), true
}

func (a *API) ListFiles(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	target, err := fsutil.SafeJoin(serverDir, r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, fileEntry{Name: e.Name(), IsDir: e.IsDir(), Size: info.Size(), ModTime: info.ModTime().Unix()})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (a *API) ReadFile(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	target, err := fsutil.SafeJoin(serverDir, r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() || info.Size() > 5*1024*1024 {
		http.Error(w, "not readable as text", http.StatusBadRequest)
		return
	}
	data, err := os.ReadFile(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

func (a *API) WriteFile(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	target, err := fsutil.SafeJoin(serverDir, body.Path)
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	if err := os.WriteFile(target, []byte(body.Content), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) DownloadFile(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	target, err := fsutil.SafeJoin(serverDir, r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, target)
}

func (a *API) UploadFile(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	r.ParseMultipartForm(64 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	destPath := filepath.Join(r.URL.Query().Get("path"), header.Filename)
	target, err := fsutil.SafeJoin(serverDir, destPath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	out, err := os.Create(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()
	io.Copy(out, file)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) MkdirFile(w http.ResponseWriter, r *http.Request) {
	a.pathAction(w, r, func(target string) error { return os.Mkdir(target, 0o755) })
}

func (a *API) CreateFileHandler(w http.ResponseWriter, r *http.Request) {
	a.pathAction(w, r, func(target string) error {
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		return f.Close()
	})
}

func (a *API) DeleteFile(w http.ResponseWriter, r *http.Request) {
	a.pathAction(w, r, func(target string) error { return os.RemoveAll(target) })
}

func (a *API) RenameFile(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	var body struct{ Path, NewName string }
	json.NewDecoder(r.Body).Decode(&body)

	target, err := fsutil.SafeJoin(serverDir, body.Path)
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	dest, err := fsutil.SafeJoin(serverDir, filepath.Join(filepath.Dir(body.Path), body.NewName))
	if err != nil {
		http.Error(w, "invalid destination", http.StatusForbidden)
		return
	}
	if _, err := os.Stat(dest); err == nil {
		http.Error(w, "destination already exists", http.StatusConflict)
		return
	}
	if err := os.Rename(target, dest); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) pathAction(w http.ResponseWriter, r *http.Request, fn func(target string) error) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	var body struct{ Path string }
	json.NewDecoder(r.Body).Decode(&body)
	target, err := fsutil.SafeJoin(serverDir, body.Path)
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	if err := fn(target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}