// internal/httpapi/projects.go
package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mcpanel/internal/mcserver"
	"mcpanel/internal/project"
	"mcpanel/internal/settings"
)

func (a *API) BuildProfilesAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		profiles, _ := project.ListProfiles(a.DB, name)
		json.NewEncoder(w).Encode(profiles)
	case http.MethodPost:
		var p project.BuildProfile
		json.NewDecoder(r.Body).Decode(&p)
		p.ProjectName = name
		if err := project.SaveProfile(a.DB, p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	case http.MethodDelete:
		project.DeleteProfile(a.DB, name, r.URL.Query().Get("profile"))
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

func (a *API) ProjectSettingsAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(project.LoadProjectSettings(a.DB, name))
		return
	}
	var ps project.ProjectSettings
	json.NewDecoder(r.Body).Decode(&ps)
	if err := project.SaveProjectSettings(a.DB, name, ps); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) StartBuild(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	profileName := r.URL.Query().Get("profile")
	if !project.IsKnownProjectDB(a.ProjectsRoot, name, a.DB) {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	profiles, _ := project.ListProfiles(a.DB, name)
	var chosen *project.BuildProfile
	for i := range profiles {
		if profiles[i].ProfileName == profileName {
			chosen = &profiles[i]
			break
		}
	}
	if chosen == nil {
		for i := range profiles {
			if profiles[i].IsDefault {
				chosen = &profiles[i]
				break
			}
		}
	}
	if chosen == nil && len(profiles) > 0 {
		chosen = &profiles[0]
	}
	if chosen == nil {
		http.Error(w, "no build profile available", http.StatusBadRequest)
		return
	}

	dir := project.ProjectDirDB(a.ProjectsRoot, name, a.DB)
	ps := project.LoadProjectSettings(a.DB, name)
	// Build settings apply to whichever build profile is selected.
	chosen.BuildArgs = ps.BuildArgs
	javaHome := ps.BuildJavaHome
	if javaHome == "" {
		javaHome = settings.LoadGlobal(a.DB).DefaultBuildJavaHome
	}
	if err := project.ValidateBuildJavaHome(javaHome); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := a.Builder.Start(dir, name, *chosen, javaHome); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) CancelBuild(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := a.Builder.Cancel(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) ListArtifactsAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !project.IsKnownProjectDB(a.ProjectsRoot, name, a.DB) {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	dir := project.ProjectDirDB(a.ProjectsRoot, name, a.DB)
	artifacts, err := project.ListArtifacts(dir)
	if err != nil {
		artifacts = []string{} // Return empty list instead of error if dir doesn't exist yet
	}
	if artifacts == nil {
		artifacts = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(artifacts)
}

// DeployProject copies the built artifact to every requested target.
// It accepts a JSON payload of: {"targets": ["server1"], "artifact": "myplugin.jar"}
// If payload is just an array, it's treated as targets for backward compatibility.
func (a *API) DeployProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	restart := r.URL.Query().Get("restart") == "true"

	if !project.IsKnownProjectDB(a.ProjectsRoot, name, a.DB) {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	dir := project.ProjectDirDB(a.ProjectsRoot, name, a.DB)
	ps := project.LoadProjectSettings(a.DB, name)

	var payload struct {
		Targets  []string `json:"targets"`
		Artifact string   `json:"artifact"`
	}

	// Read payload
	err := json.NewDecoder(r.Body).Decode(&payload)
	var targets []string
	if err == nil {
		targets = payload.Targets
	}

	// If it was just an array previously, it would fail the struct decode, so check if we got targets
	if len(targets) == 0 {
		targets = ps.DeployTargets // fallback to saved settings
	}

	var artifactPath string
	if payload.Artifact != "" {
		clean := filepath.Clean(filepath.FromSlash(payload.Artifact))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			http.Error(w, "invalid artifact path", http.StatusBadRequest)
			return
		}
		artifactPath = filepath.Join(dir, clean)
		if info, err := os.Stat(artifactPath); err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(artifactPath), ".jar") {
			http.Error(w, "selected artifact does not exist", http.StatusBadRequest)
			return
		}
	} else {
		artifactPath, err = project.DetectArtifact(dir, ps.ArtifactGlob)
		if err != nil {
			http.Error(w, "no build artifact found — build the project first", http.StatusBadRequest)
			return
		}
	}

	destName := ps.DeploymentFilename
	if destName == "" {
		destName = filepath.Base(artifactPath)
	}

	results := map[string]string{}
	for _, serverName := range targets {
		if !mcserver.IsKnownServer(a.MCRoot, serverName, a.ServerSubfolders()) {
			results[serverName] = "unknown server"
			continue
		}
		serverDir := mcserver.ServerDir(a.MCRoot, serverName)
		if err := project.Deploy(a.DB, name, serverName, serverDir, artifactPath, destName, 0); err != nil {
			results[serverName] = "failed: " + err.Error()
			continue
		}
		results[serverName] = "deployed"
		if restart {
			s := a.SettingsFor(serverName)
			if err := a.Screen.Restart(serverName, serverDir, s, 30*time.Second); err != nil {
				results[serverName] += ", restart failed: " + err.Error()
			} else {
				results[serverName] += ", restarted"
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// ── Custom Projects API ───────────────────────────────────────────────────

func (a *API) CustomProjectsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			Name string `json:"name"`
			Dir  string `json:"dir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Dir == "" {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		// Validate dir exists and is a directory
		info, err := os.Stat(payload.Dir)
		if err != nil || !info.IsDir() {
			http.Error(w, "directory does not exist", http.StatusBadRequest)
			return
		}
		// Derive name from directory basename if not provided
		if payload.Name == "" {
			payload.Name = filepath.Base(payload.Dir)
		}
		// Sanitize name
		payload.Name = strings.ReplaceAll(payload.Name, "/", "_")
		payload.Name = strings.ReplaceAll(payload.Name, "\\", "_")
		if err := project.AddCustomProject(a.DB, payload.Name, payload.Dir); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"ok": "true", "name": payload.Name})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if err := project.RemoveCustomProject(a.DB, name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}
}

// ── System Utilities ──────────────────────────────────────────────────────

func (a *API) PickFolderAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	out, err := exec.Command("osascript", "-e", "POSIX path of (choose folder)").Output()
	if err != nil {
		http.Error(w, "folder selection cancelled or failed", http.StatusBadRequest)
		return
	}
	path := strings.TrimSpace(string(out))
	json.NewEncoder(w).Encode(map[string]string{"path": path})
}
