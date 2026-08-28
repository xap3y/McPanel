package httpapi

import (
	"bytes"
	"strings"
	"testing"

	"mcpanel/internal/project"
)

func TestProjectPageRendersPastDeployTargets(t *testing.T) {
	var out bytes.Buffer
	err := projectTpl.ExecuteTemplate(&out, "layout", map[string]any{
		"Name":       "Example",
		"Servers":    []string{"server-one"},
		"AllServers": []sidebarServer{},
		"Profiles":   []project.BuildProfile{},
		"Settings":   project.ProjectSettings{},
		"CSRF":       "test-token",
		"ActiveNav":  "projects",
		"IsDefaultTarget": func(name string) bool {
			return name == "server-one"
		},
	})
	if err != nil {
		t.Fatalf("render project page: %v", err)
	}
	page := out.String()
	for _, expected := range []string{"Build Profiles", "Add Custom Command", "artifact-select", "target-checkbox", "deploy-restart-btn", "server-context-menu", "data-server-menu-action=\"rename\"", "style.css?v=17", "project.js?v=10"} {
		if !strings.Contains(page, expected) {
			t.Errorf("rendered page is missing %q", expected)
		}
	}
}

func TestNewServerPageRendersCreationOptions(t *testing.T) {
	var out bytes.Buffer
	err := newServerTpl.ExecuteTemplate(&out, "layout", map[string]any{
		"Groups":     []string{"folia"},
		"AllServers": []sidebarServer{},
		"CSRF":       "test-token",
		"ActiveNav":  "new-server",
	})
	if err != nil {
		t.Fatalf("render new server page: %v", err)
	}
	page := out.String()
	for _, expected := range []string{"Create Server", "Server JAR URL", "folia", "settings.allow-end", "view-distance", "online-mode", "allow-nether", "level-type", "new-server.js?v=1"} {
		if !strings.Contains(page, expected) {
			t.Errorf("rendered page is missing %q", expected)
		}
	}
}

func TestDashboardRendersBulkAndHealthControls(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTpl.ExecuteTemplate(&out, "layout", map[string]any{
		"Servers": []dashboardServerRow{{Name: "folia/1.21.11"}},
		"CSRF":    "test-token",
	})
	if err != nil {
		t.Fatalf("render dashboard: %v", err)
	}
	page := out.String()
	for _, expected := range []string{"select-all-servers", "data-bulk-action=\"backup\"", "data-bulk-action=\"delete\"", "card-disk-folia/1.21.11", "card-tps-folia/1.21.11", "card-players-folia/1.21.11"} {
		if !strings.Contains(page, expected) {
			t.Errorf("rendered dashboard is missing %q", expected)
		}
	}
}

func TestValidServerDirectoryName(t *testing.T) {
	for _, name := range []string{"1.21.11", "folia-test", "server_2"} {
		if !validServerDirectoryName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}
	for _, name := range []string{"", ".hidden", "../escape", "two/levels", "has space"} {
		if validServerDirectoryName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestServerPageRendersEmbeddedMonacoEditor(t *testing.T) {
	var out bytes.Buffer
	err := serverTpl.ExecuteTemplate(&out, "layout", map[string]any{
		"Name":       "folia/1.21.11",
		"AllServers": []sidebarServer{},
		"CSRF":       "test-token",
		"ActiveNav":  "server-folia/1.21.11",
	})
	if err != nil {
		t.Fatalf("render server page: %v", err)
	}
	page := out.String()
	for _, expected := range []string{"file-editor-workspace", "monaco-editor", "editor-back-btn", "editor-save-btn", "health-disk", "s-port", "startup-command", "console-search", "console.js?v=3", "vendor/monaco/vs/loader.js", "server.js?v=11"} {
		if !strings.Contains(page, expected) {
			t.Errorf("rendered page is missing %q", expected)
		}
	}
	if strings.Contains(page, "editor-modal") {
		t.Error("server page still renders the old floating editor modal")
	}
}
