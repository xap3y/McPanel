package httpapi

import (
	"encoding/json"
	"net/http"

	"mcpanel/internal/mcserver"
	"mcpanel/internal/settings"
)

func (a *API) ServerSettingsAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(settings.Load(a.DB, name))
		return
	}
	var s mcserver.Settings
	json.NewDecoder(r.Body).Decode(&s)
	if err := settings.Save(a.DB, name, s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) VelocityToggle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	var body struct{ Enable bool }
	json.NewDecoder(r.Body).Decode(&body)

	dir := mcserver.ServerDir(a.MCRoot, name)
	if err := mcserver.SetVelocity(dir, body.Enable); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s := settings.Load(a.DB, name)
	s.VelocityEnabled = body.Enable
	settings.Save(a.DB, name, s)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}