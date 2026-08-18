package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mcpanel/internal/mcserver"
	"mcpanel/internal/project"
	"mcpanel/internal/settings"
)

// ── Template function maps ─────────────────────────────────────────────────

var tplFuncs = template.FuncMap{
	"lower": func(v interface{}) string {
		return strings.ToLower(fmt.Sprintf("%v", v))
	},
	"basename":  filepath.Base,
	"fmtRAM":    fmtRAM,
	"fmtUptime": fmtUptime,
	"fmtTime":   fmtRelTime,
}

func fmtRAM(bytes uint64) string {
	if bytes == 0 {
		return "0 B"
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	if gb >= 1 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	mb := float64(bytes) / (1024 * 1024)
	return fmt.Sprintf("%.0f MB", mb)
}

func fmtUptime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func fmtRelTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return t.Format("Jan 2")
}

// ── Template parsing helpers ───────────────────────────────────────────────

func parseTpl(files ...string) *template.Template {
	resolved := make([]string, len(files))
	for i, file := range files {
		resolved[i] = file
		if _, err := os.Stat(file); os.IsNotExist(err) {
			// `go test` runs package tests from internal/httpapi, while the
			// panel itself runs from the repository root.
			resolved[i] = filepath.Join("..", "..", file)
		}
	}
	return template.Must(template.New("layout").Funcs(tplFuncs).ParseFiles(resolved...))
}

// ── Templates ─────────────────────────────────────────────────────────────

var (
	dashboardTpl = parseTpl("web/templates/layout.html", "web/templates/dashboard.html")
	serverTpl    = parseTpl("web/templates/layout.html", "web/templates/server.html")
	projectsTpl  = parseTpl("web/templates/layout.html", "web/templates/projects.html")
	projectTpl   = parseTpl("web/templates/layout.html", "web/templates/project.html")
)

// ── Dashboard ─────────────────────────────────────────────────────────────

type dashboardServerRow struct {
	Name      string
	Status    mcserver.Status
	PID       int
	StartedAt time.Time
	RAMBytes  uint64
	CPUPct    float64
}

type buildSummary struct {
	Status       string
	StartedAt    time.Time
	ArtifactPath string
}

type dashboardProjectRow struct {
	Name      string
	LastBuild *buildSummary
}

func (a *API) Dashboard(w http.ResponseWriter, r *http.Request) {
	names, err := mcserver.Discover(a.MCRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]dashboardServerRow, 0, len(names))
	for _, n := range names {
		running, pid := a.Screen.IsRunning(n)
		st := mcserver.StatusOffline
		if running {
			st = mcserver.StatusOnline
		}
		row := dashboardServerRow{Name: n, Status: st, PID: pid}
		if running && pid > 0 {
			row.RAMBytes, row.CPUPct = mcserver.SampleResource(pid)
			row.StartedAt = a.Screen.StartedAt(n)
		}
		rows = append(rows, row)
	}

	projs, _ := project.Discover(a.ProjectsRoot)
	projRows := make([]dashboardProjectRow, 0, len(projs))
	for _, p := range projs {
		row := dashboardProjectRow{Name: p.Name}
		if a.DB != nil {
			var status, artifact string
			var startedAt time.Time
			if err2 := a.DB.QueryRow(
				`SELECT status, started_at, COALESCE(artifact_path,'') FROM build_history WHERE project_name=? ORDER BY id DESC LIMIT 1`,
				p.Name,
			).Scan(&status, &startedAt, &artifact); err2 == nil {
				row.LastBuild = &buildSummary{Status: status, StartedAt: startedAt, ArtifactPath: artifact}
			}
		}
		projRows = append(projRows, row)
	}

	dashboardTpl.ExecuteTemplate(w, "layout", map[string]any{
		"Servers":   rows,
		"Projects":  projRows,
		"CSRF":      sessionFrom(r).CSRF,
		"ActiveNav": "dashboard",
	})
}

// ── Sidebar helper ────────────────────────────────────────────────────────

type sidebarServer struct {
	Name   string
	Status mcserver.Status
}

func (a *API) allServersForSidebar() []sidebarServer {
	names, _ := mcserver.Discover(a.MCRoot)
	out := make([]sidebarServer, 0, len(names))
	for _, n := range names {
		running, _ := a.Screen.IsRunning(n)
		st := mcserver.StatusOffline
		if running {
			st = mcserver.StatusOnline
		}
		out = append(out, sidebarServer{Name: n, Status: st})
	}
	return out
}

// ── Server page ───────────────────────────────────────────────────────────

func (a *API) ServerPage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name) {
		http.NotFound(w, r)
		return
	}
	serverTpl.ExecuteTemplate(w, "layout", map[string]any{
		"Name":       name,
		"CSRF":       sessionFrom(r).CSRF,
		"AllServers": a.allServersForSidebar(),
		"ActiveNav":  "server-" + name,
	})
}

// ── Server action ─────────────────────────────────────────────────────────

func (a *API) ServerAction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	action := r.PathValue("action")

	if !mcserver.IsKnownServer(a.MCRoot, name) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	dir := mcserver.ServerDir(a.MCRoot, name)
	s := settings.Load(a.DB, name)

	var err error
	switch action {
	case "start":
		err = a.Screen.Start(name, dir, s)
	case "stop":
		err = a.Screen.Stop(name, 30*time.Second)
	case "restart":
		err = a.Screen.Restart(name, dir, s, 30*time.Second)
	case "kill":
		err = a.Screen.Kill(name)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// ── Server status API (polled by topbar) ─────────────────────────────────

func (a *API) ServerStatusAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	running, pid := a.Screen.IsRunning(name)
	st := "OFFLINE"
	uptime := ""
	ramStr := ""
	if running {
		st = "ONLINE"
		startedAt := a.Screen.StartedAt(name)
		if !startedAt.IsZero() {
			uptime = fmtUptime(startedAt)
		}
		if pid > 0 {
			ram, _ := mcserver.SampleResource(pid)
			if ram > 0 {
				ramStr = fmtRAM(ram)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": st,
		"pid":    pid,
		"uptime": uptime,
		"ram":    ramStr,
	})
}

// ── Projects page ─────────────────────────────────────────────────────────

func (a *API) ProjectsPage(w http.ResponseWriter, r *http.Request) {
	projs, err := project.Discover(a.ProjectsRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type projRow struct {
		Name      string
		LastBuild *buildSummary
	}
	rows := make([]projRow, 0, len(projs))
	for _, p := range projs {
		row := projRow{Name: p.Name}
		if a.DB != nil {
			var status, artifact string
			var startedAt time.Time
			if err2 := a.DB.QueryRow(
				`SELECT status, started_at, COALESCE(artifact_path,'') FROM build_history WHERE project_name=? ORDER BY id DESC LIMIT 1`,
				p.Name,
			).Scan(&status, &startedAt, &artifact); err2 == nil {
				row.LastBuild = &buildSummary{Status: status, StartedAt: startedAt, ArtifactPath: artifact}
			}
		}
		rows = append(rows, row)
	}

	projectsTpl.ExecuteTemplate(w, "layout", map[string]any{
		"Projects":  rows,
		"Servers":   a.allServersForSidebar(),
		"CSRF":      sessionFrom(r).CSRF,
		"ActiveNav": "projects",
	})
}

// ── Project page ──────────────────────────────────────────────────────────

func (a *API) ProjectPage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !project.IsKnownProject(a.ProjectsRoot, name) {
		http.NotFound(w, r)
		return
	}
	serverNames, _ := mcserver.Discover(a.MCRoot)
	profiles, _ := project.ListProfiles(a.DB, name)
	ps := project.LoadProjectSettings(a.DB, name)

	defaultTargets := make(map[string]bool)
	for _, t := range ps.DeployTargets {
		defaultTargets[t] = true
	}

	projectTpl.ExecuteTemplate(w, "layout", map[string]any{
		"Name":            name,
		"Servers":         serverNames,
		"AllServers":      a.allServersForSidebar(),
		"Profiles":        profiles,
		"Settings":        ps,
		"CSRF":            sessionFrom(r).CSRF,
		"ActiveNav":       "projects",
		"IsDefaultTarget": func(s string) bool { return defaultTargets[s] },
	})
}

// ── Build history API ─────────────────────────────────────────────────────

type buildHistoryRow struct {
	ID           int64
	ProjectName  string
	ProfileName  string
	Status       string
	StartedAt    time.Time
	FinishedAt   *time.Time
	ExitCode     *int
	ArtifactPath string
}

func (a *API) BuildHistoryAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	limit := 20
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	dbRows, err := a.DB.Query(
		`SELECT id, project_name, profile_name, status, started_at,
		        finished_at, exit_code, COALESCE(artifact_path,'')
		 FROM build_history WHERE project_name=? ORDER BY id DESC LIMIT ?`,
		name, limit,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}
	defer dbRows.Close()

	var out []buildHistoryRow
	for dbRows.Next() {
		var row buildHistoryRow
		var finishedAt sql.NullTime
		var exitCode sql.NullInt32
		if err := dbRows.Scan(&row.ID, &row.ProjectName, &row.ProfileName, &row.Status, &row.StartedAt,
			&finishedAt, &exitCode, &row.ArtifactPath); err != nil {
			continue
		}
		if finishedAt.Valid {
			row.FinishedAt = &finishedAt.Time
		}
		if exitCode.Valid {
			n := int(exitCode.Int32)
			row.ExitCode = &n
		}
		out = append(out, row)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// ── Deploy history API ────────────────────────────────────────────────────

type deployHistoryRow struct {
	ID           int64
	ProjectName  string
	ServerName   string
	BuildID      *int64
	Status       string
	Restarted    bool
	DeployedAt   time.Time
	ErrorMessage string
}

func (a *API) DeployHistoryAPI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	limit := 20
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	dbRows, err := a.DB.Query(
		`SELECT id, project_name, server_name, build_id, status, restarted, deployed_at, COALESCE(error_message,'')
		 FROM deploy_history WHERE project_name=? ORDER BY id DESC LIMIT ?`,
		name, limit,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}
	defer dbRows.Close()

	var out []deployHistoryRow
	for dbRows.Next() {
		var row deployHistoryRow
		var buildID sql.NullInt64
		if err := dbRows.Scan(&row.ID, &row.ProjectName, &row.ServerName, &buildID,
			&row.Status, &row.Restarted, &row.DeployedAt, &row.ErrorMessage); err != nil {
			continue
		}
		if buildID.Valid {
			row.BuildID = &buildID.Int64
		}
		out = append(out, row)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
