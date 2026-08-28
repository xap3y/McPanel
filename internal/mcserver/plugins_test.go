package mcserver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListPluginsIncludesJarSize(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Enabled.jar"), make([]byte, 1536), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Disabled.jar.d"), make([]byte, 2*1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := ListPlugins(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}
	sizes := map[string]int64{}
	for _, plugin := range plugins {
		sizes[plugin.Name] = plugin.SizeBytes
	}
	if sizes["Enabled.jar"] != 1536 || sizes["Disabled.jar"] != 2*1024*1024 {
		t.Fatalf("unexpected plugin sizes: %#v", sizes)
	}
}
