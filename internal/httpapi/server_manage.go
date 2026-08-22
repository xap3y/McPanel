package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mcpanel/internal/fsutil"
	"mcpanel/internal/mcserver"
)

type manageServerRequest struct {
	Action  string `json:"action"`
	Name    string `json:"name"`
	NewName string `json:"newName"`
}

func (a *API) ServerInfoAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name, a.ServerSubfolders()) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"name": name,
		"path": mcserver.ServerDir(a.MCRoot, name),
	})
}

func (a *API) ManageServer(w http.ResponseWriter, r *http.Request) {
	var req manageServerRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !mcserver.IsKnownServer(a.MCRoot, req.Name, a.ServerSubfolders()) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	if running, _ := a.Screen.IsRunning(req.Name); running {
		http.Error(w, "stop the server before renaming or deleting it", http.StatusConflict)
		return
	}

	switch req.Action {
	case "rename":
		a.renameServer(w, req)
	case "delete":
		a.deleteServer(w, req.Name)
	default:
		http.Error(w, "unknown server management action", http.StatusBadRequest)
	}
}

func (a *API) renameServer(w http.ResponseWriter, req manageServerRequest) {
	newBase := strings.TrimSpace(req.NewName)
	if !validServerDirectoryName(newBase) {
		http.Error(w, "new name may only contain letters, numbers, dots, underscores, and hyphens", http.StatusBadRequest)
		return
	}
	group := ""
	if slash := strings.LastIndex(req.Name, "/"); slash >= 0 {
		group = req.Name[:slash]
	}
	newServerName := newBase
	if group != "" {
		newServerName = group + "/" + newBase
	}
	if newServerName == req.Name {
		writeServerManagementResult(w, newServerName)
		return
	}
	targetDir, err := fsutil.SafeJoin(a.MCRoot, filepath.FromSlash(newServerName))
	if err != nil {
		http.Error(w, "invalid destination path", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(targetDir); err == nil {
		http.Error(w, "a server with that name already exists", http.StatusConflict)
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sourceDir := mcserver.ServerDir(a.MCRoot, req.Name)
	if err := os.Rename(sourceDir, targetDir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// These records are metadata; a filesystem rename remains successful even
	// if an older installation does not contain every table/column.
	if a.DB != nil {
		_, _ = a.DB.Exec(`UPDATE server_settings SET server_name=? WHERE server_name=?`, newServerName, req.Name)
		_, _ = a.DB.Exec(`UPDATE deploy_history SET server_name=? WHERE server_name=?`, newServerName, req.Name)
		a.replaceProjectDeployTarget(req.Name, newServerName)
	}
	writeServerManagementResult(w, newServerName)
}

func (a *API) deleteServer(w http.ResponseWriter, name string) {
	dir := mcserver.ServerDir(a.MCRoot, name)
	if err := os.RemoveAll(dir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if a.DB != nil {
		_, _ = a.DB.Exec(`DELETE FROM server_settings WHERE server_name=?`, name)
		a.replaceProjectDeployTarget(name, "")
	}
	writeServerManagementResult(w, name)
}

func (a *API) replaceProjectDeployTarget(oldName, newName string) {
	rows, err := a.DB.Query(`SELECT project_name, COALESCE(deploy_targets,'') FROM project_settings WHERE deploy_targets IS NOT NULL`)
	if err != nil {
		return
	}
	type targetUpdate struct{ project, targets string }
	var updates []targetUpdate
	for rows.Next() {
		var projectName, targets string
		if rows.Scan(&projectName, &targets) != nil {
			continue
		}
		parts := strings.Split(targets, ",")
		changed := false
		kept := parts[:0]
		for _, part := range parts {
			if part == oldName {
				changed = true
				if newName != "" {
					kept = append(kept, newName)
				}
				continue
			}
			kept = append(kept, part)
		}
		if changed {
			updates = append(updates, targetUpdate{projectName, strings.Join(kept, ",")})
		}
	}
	_ = rows.Close()
	for _, update := range updates {
		_, _ = a.DB.Exec(`UPDATE project_settings SET deploy_targets=? WHERE project_name=?`, update.targets, update.project)
	}
}

func writeServerManagementResult(w http.ResponseWriter, name string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "name": name})
}
