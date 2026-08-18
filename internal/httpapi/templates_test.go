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
	for _, expected := range []string{"Build Profiles", "Add Custom Command", "artifact-picker", "target-checkbox", "project.js?v=5"} {
		if !strings.Contains(page, expected) {
			t.Errorf("rendered page is missing %q", expected)
		}
	}
}
