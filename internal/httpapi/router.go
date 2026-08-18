package httpapi

import (
	"database/sql"
	"net/http"

	"mcpanel/internal/auth"
	"mcpanel/internal/mcserver"
	"mcpanel/internal/project"
	"mcpanel/internal/settings"
)

type API struct {
	MCRoot       string
	ProjectsRoot string
	DB           *sql.DB
	Auth         *auth.Store
	Limiter      *auth.Limiter
	Screen       *mcserver.Manager
	Console      *mcserver.ConsoleManager
	Builder      *project.Runner
}

func (a *API) SettingsFor(name string) mcserver.Settings {
	return settings.Load(a.DB, name)
}

func NewRouter(a *API) http.Handler {
	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("GET /login", a.LoginPage)
	mux.HandleFunc("POST /login", a.LoginSubmit)
	mux.HandleFunc("POST /logout", a.requireAuth(a.Logout))

	// Dashboard
	mux.HandleFunc("GET /{$}", a.requireAuth(a.Dashboard))

	// Server pages & actions
	mux.HandleFunc("GET /servers/{name}", a.requireAuth(a.ServerPage))
	mux.HandleFunc("POST /api/servers/{name}/{action}", a.requireAuth(a.requireCSRF(a.ServerAction)))
	mux.HandleFunc("GET /api/servers/{name}/status", a.requireAuth(a.ServerStatusAPI))
	mux.HandleFunc("GET /ws/console/{name}", a.requireAuth(a.ConsoleWS))

	// File manager
	mux.HandleFunc("GET /api/servers/{name}/files", a.requireAuth(a.ListFiles))
	mux.HandleFunc("GET /api/servers/{name}/files/read", a.requireAuth(a.ReadFile))
	mux.HandleFunc("POST /api/servers/{name}/files/write", a.requireAuth(a.requireCSRF(a.WriteFile)))
	mux.HandleFunc("GET /api/servers/{name}/files/download", a.requireAuth(a.DownloadFile))
	mux.HandleFunc("POST /api/servers/{name}/files/upload", a.requireAuth(a.requireCSRF(a.UploadFile)))
	mux.HandleFunc("POST /api/servers/{name}/files/mkdir", a.requireAuth(a.requireCSRF(a.MkdirFile)))
	mux.HandleFunc("POST /api/servers/{name}/files/create", a.requireAuth(a.requireCSRF(a.CreateFileHandler)))
	mux.HandleFunc("POST /api/servers/{name}/files/delete", a.requireAuth(a.requireCSRF(a.DeleteFile)))
	mux.HandleFunc("POST /api/servers/{name}/files/rename", a.requireAuth(a.requireCSRF(a.RenameFile)))

	// Plugins
	mux.HandleFunc("GET /api/servers/{name}/plugins", a.requireAuth(a.ListPlugins))
	mux.HandleFunc("POST /api/servers/{name}/plugins/{plugin}/toggle", a.requireAuth(a.requireCSRF(a.TogglePlugin)))
	mux.HandleFunc("POST /api/servers/{name}/plugins/{plugin}/delete", a.requireAuth(a.requireCSRF(a.DeletePlugin)))

	// Server settings & Velocity
	mux.HandleFunc("GET /api/servers/{name}/settings", a.requireAuth(a.ServerSettingsAPI))
	mux.HandleFunc("POST /api/servers/{name}/settings", a.requireAuth(a.requireCSRF(a.ServerSettingsAPI)))
	mux.HandleFunc("POST /api/servers/{name}/velocity", a.requireAuth(a.requireCSRF(a.VelocityToggle)))

	// Project pages
	mux.HandleFunc("GET /projects", a.requireAuth(a.ProjectsPage))
	mux.HandleFunc("GET /projects/{name}", a.requireAuth(a.ProjectPage))

	// Project APIs
	mux.HandleFunc("GET /api/projects/{name}/profiles", a.requireAuth(a.BuildProfilesAPI))
	mux.HandleFunc("POST /api/projects/{name}/profiles", a.requireAuth(a.requireCSRF(a.BuildProfilesAPI)))
	mux.HandleFunc("DELETE /api/projects/{name}/profiles", a.requireAuth(a.requireCSRF(a.BuildProfilesAPI)))
	mux.HandleFunc("GET /api/projects/{name}/settings", a.requireAuth(a.ProjectSettingsAPI))
	mux.HandleFunc("POST /api/projects/{name}/settings", a.requireAuth(a.requireCSRF(a.ProjectSettingsAPI)))
	mux.HandleFunc("POST /api/projects/{name}/build", a.requireAuth(a.requireCSRF(a.StartBuild)))
	mux.HandleFunc("POST /api/projects/{name}/cancel", a.requireAuth(a.requireCSRF(a.CancelBuild)))
	mux.HandleFunc("POST /api/projects/{name}/deploy", a.requireAuth(a.requireCSRF(a.DeployProject)))
	mux.HandleFunc("GET /api/projects/{name}/artifacts", a.requireAuth(a.ListArtifactsAPI))
	mux.HandleFunc("GET /ws/build/{name}", a.requireAuth(a.BuildWS))

	// History APIs
	mux.HandleFunc("GET /api/projects/{name}/history/builds", a.requireAuth(a.BuildHistoryAPI))
	mux.HandleFunc("GET /api/projects/{name}/history/deploys", a.requireAuth(a.DeployHistoryAPI))

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	return mux
}
