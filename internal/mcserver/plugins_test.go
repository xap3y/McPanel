package mcserver

import (
	"bytes"
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

func TestUploadPluginAcceptsOnlyNewJARFiles(t *testing.T) {
	server := t.TempDir()
	if err := UploadPlugin(server, "Example.JAR", bytes.NewBufferString("plugin bytes")); err != nil {
		t.Fatalf("upload plugin: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(server, "plugins", "Example.JAR"))
	if err != nil || string(got) != "plugin bytes" {
		t.Fatalf("uploaded plugin = %q, %v", got, err)
	}
	if err := UploadPlugin(server, "Example.JAR", bytes.NewBufferString("replacement")); err == nil {
		t.Fatal("expected duplicate plugin upload to be rejected")
	}
	if err := UploadPlugin(server, "notes.txt", bytes.NewBufferString("not a plugin")); err == nil {
		t.Fatal("expected non-JAR upload to be rejected")
	}
}

func TestCopyPluginPreservesStateAndDoesNotOverwrite(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	pluginPath := filepath.Join(source, "plugins", "Example.jar.d")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, []byte("plugin bytes"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := CopyPlugin(source, destination, "Example.jar", false); err != nil {
		t.Fatalf("copy plugin: %v", err)
	}
	copied, err := os.ReadFile(filepath.Join(destination, "plugins", "Example.jar.d"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != "plugin bytes" {
		t.Fatalf("copied content = %q", copied)
	}
	if err := CopyPlugin(source, destination, "Example.jar", false); err == nil {
		t.Fatal("expected existing destination plugin to be rejected")
	}
}
