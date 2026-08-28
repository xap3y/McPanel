package httpapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mcpanel/internal/mcserver"
	"mcpanel/internal/settings"
)

func (a *API) ServerSettingsAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name, a.ServerSubfolders()) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		s := settings.Load(a.DB, name)
		s.ServerPort = readServerPort(mcserver.ServerDir(a.MCRoot, name))
		json.NewEncoder(w).Encode(s)
		return
	}
	var s mcserver.Settings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "invalid settings", http.StatusBadRequest)
		return
	}
	if s.ServerPort < 1 || s.ServerPort > 65535 {
		http.Error(w, "server port must be between 1 and 65535", http.StatusBadRequest)
		return
	}
	if err := writeServerPort(mcserver.ServerDir(a.MCRoot, name), s.ServerPort); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := settings.Save(a.DB, name, s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func readServerPort(serverDir string) int {
	file, err := os.Open(filepath.Join(serverDir, "server.properties"))
	if err != nil {
		return 25565
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "server-port=") {
			if port, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "server-port="))); err == nil {
				return port
			}
		}
	}
	return 25565
}

func writeServerPort(serverDir string, port int) error {
	path := filepath.Join(serverDir, "server.properties")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "server-port=") {
			lines[i] = fmt.Sprintf("server-port=%d", port)
			found = true
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("server-port=%d", port))
	}
	return os.WriteFile(path, []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n")), 0o644)
}

func (a *API) VelocityToggle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name, a.ServerSubfolders()) {
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

// ── Global settings ───────────────────────────────────────────────────────

var globalSettingsTpl = parseTpl("web/templates/layout.html", "web/templates/global_settings.html")

func (a *API) GlobalSettingsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Subfolders []string
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g := settings.GlobalSettings{Subfolders: req.Subfolders}
		if err := settings.SaveGlobal(a.DB, g); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	g := settings.LoadGlobal(a.DB)
	globalSettingsTpl.ExecuteTemplate(w, "layout", map[string]any{
		"Subfolders": strings.Join(g.Subfolders, ", "),
		"AllServers": a.allServersForSidebar(),
		"CSRF":       sessionFrom(r).CSRF,
		"ActiveNav":  "settings",
	})
}
