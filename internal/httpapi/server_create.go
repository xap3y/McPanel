package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"mcpanel/internal/fsutil"
	"mcpanel/internal/mcserver"
)

var newServerTpl = parseTpl("web/templates/layout.html", "web/templates/new_server.html")

type createServerRequest struct {
	Name             string            `json:"name"`
	Group            string            `json:"group"`
	JarURL           string            `json:"jarUrl"`
	JarFilename      string            `json:"jarFilename"`
	AcceptEULA       bool              `json:"acceptEula"`
	ServerProperties map[string]string `json:"serverProperties"`
	BukkitSettings   map[string]string `json:"bukkitSettings"`
}

func (a *API) NewServerPage(w http.ResponseWriter, r *http.Request) {
	newServerTpl.ExecuteTemplate(w, "layout", map[string]any{
		"Groups":     a.ServerSubfolders(),
		"AllServers": a.allServersForSidebar(),
		"CSRF":       sessionFrom(r).CSRF,
		"ActiveNav":  "new-server",
	})
}

func (a *API) CreateServer(w http.ResponseWriter, r *http.Request) {
	var req createServerRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Group = strings.TrimSpace(req.Group)
	if !validServerDirectoryName(req.Name) {
		http.Error(w, "server name may only contain letters, numbers, dots, underscores, and hyphens", http.StatusBadRequest)
		return
	}
	if req.Group != "" && !containsString(a.ServerSubfolders(), req.Group) {
		http.Error(w, "unknown server group", http.StatusBadRequest)
		return
	}

	relative := req.Name
	if req.Group != "" {
		relative = filepath.Join(req.Group, req.Name)
	}
	targetDir, err := fsutil.SafeJoin(a.MCRoot, relative)
	if err != nil {
		http.Error(w, "invalid server path", http.StatusBadRequest)
		return
	}
	err = mcserver.CreateServer(r.Context(), nil, targetDir, mcserver.CreateOptions{
		JarURL:           strings.TrimSpace(req.JarURL),
		JarFilename:      strings.TrimSpace(req.JarFilename),
		AcceptEULA:       req.AcceptEULA,
		ServerProperties: req.ServerProperties,
		BukkitSettings:   req.BukkitSettings,
	})
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, mcserver.ErrServerExists) {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "URL") {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	serverName := filepath.ToSlash(relative)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"name": serverName})
}

func validServerDirectoryName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
