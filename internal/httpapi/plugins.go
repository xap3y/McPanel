package httpapi

import (
	"encoding/json"
	"net/http"

	"mcpanel/internal/mcserver"
)

func (a *API) UploadPlugin(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		http.Error(w, "plugin upload must be 256 MB or smaller", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "choose a plugin JAR to upload", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if err := mcserver.UploadPlugin(serverDir, header.Filename, file); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) ListPlugins(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	plugins, err := mcserver.ListPlugins(serverDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plugins)
}

func (a *API) TogglePlugin(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	name := r.PathValue("plugin")
	var body struct{ Enable bool }
	json.NewDecoder(r.Body).Decode(&body)

	if err := mcserver.TogglePlugin(serverDir, name, body.Enable); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) DeletePlugin(w http.ResponseWriter, r *http.Request) {
	serverDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := mcserver.DeletePlugin(serverDir, r.PathValue("plugin"), body.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) CopyPlugin(w http.ResponseWriter, r *http.Request) {
	sourceDir, ok := a.serverDirOr404(w, r)
	if !ok {
		return
	}
	var body struct {
		Destination string `json:"destination"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Destination == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.Destination == r.PathValue("name") {
		http.Error(w, "choose a different destination server", http.StatusBadRequest)
		return
	}
	if !mcserver.IsKnownServer(a.MCRoot, body.Destination, a.ServerSubfolders()) {
		http.Error(w, "unknown destination server", http.StatusNotFound)
		return
	}
	if err := mcserver.CopyPlugin(sourceDir, mcserver.ServerDir(a.MCRoot, body.Destination), r.PathValue("plugin"), body.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
