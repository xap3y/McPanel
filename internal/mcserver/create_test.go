package mcserver

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateServerDownloadsJarAndWritesSelectedConfig(t *testing.T) {
	var jar bytes.Buffer
	zw := zip.NewWriter(&jar)
	entry, err := zw.Create("META-INF/MANIFEST.MF")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("Manifest-Version: 1.0\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/java-archive")
		_, _ = w.Write(jar.Bytes())
	}))
	defer download.Close()

	root := t.TempDir()
	group := filepath.Join(root, "folia")
	if err := os.Mkdir(group, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(group, "1.21.11")
	err = CreateServer(context.Background(), download.Client(), target, CreateOptions{
		JarURL:      download.URL + "/folia.jar",
		JarFilename: "folia.jar",
		AcceptEULA:  true,
		ServerProperties: map[string]string{
			"view-distance": "32",
			"online-mode":   "false",
			"allow-nether":  "false",
			"level-type":    "minecraft:flat",
		},
		BukkitSettings: map[string]string{"settings.allow-end": "false"},
	})
	if err != nil {
		t.Fatalf("CreateServer() error: %v", err)
	}

	assertFileContains(t, filepath.Join(target, "eula.txt"), "eula=true")
	assertFileContains(t, filepath.Join(target, "server.properties"), "view-distance=32")
	assertFileContains(t, filepath.Join(target, "server.properties"), "level-type=minecraft:flat")
	assertFileContains(t, filepath.Join(target, "bukkit.yml"), "allow-end: false")
	if info, err := os.Stat(filepath.Join(target, "folia.jar")); err != nil || info.Size() == 0 {
		t.Fatalf("downloaded JAR missing or empty: info=%v err=%v", info, err)
	}
	entries, err := os.ReadDir(group)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "-creating-") {
			t.Fatalf("temporary creation directory was not cleaned up: %s", entry.Name())
		}
	}
}

func TestCreateServerRejectsNonJarAndLeavesNoServer(t *testing.T) {
	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not a jar"))
	}))
	defer download.Close()

	target := filepath.Join(t.TempDir(), "broken")
	err := CreateServer(context.Background(), download.Client(), target, CreateOptions{JarURL: download.URL, JarFilename: "server.jar"})
	if err == nil || !strings.Contains(err.Error(), "not a valid JAR") {
		t.Fatalf("CreateServer() error = %v, want invalid JAR error", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("failed creation left target directory behind: %v", statErr)
	}
}

func assertFileContains(t *testing.T, path, wanted string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), wanted) {
		t.Fatalf("%s does not contain %q:\n%s", path, wanted, content)
	}
}
