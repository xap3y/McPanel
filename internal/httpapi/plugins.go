package httpapi

import (
	"encoding/json"
	"net/http"

	"mcpanel/internal/mcserver"
)

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
