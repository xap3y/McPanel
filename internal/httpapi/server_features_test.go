package httpapi

import (
	"os"
	"path/filepath"
	"testing"

	"mcpanel/internal/mcserver"
)

func TestServerPortRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	if err := os.WriteFile(path, []byte("motd=Test\nserver-port=25565\nonline-mode=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeServerPort(dir, 25570); err != nil {
		t.Fatal(err)
	}
	if got := readServerPort(dir); got != 25570 {
		t.Fatalf("server port = %d, want 25570", got)
	}
}

func TestObservableServerStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.log")
	log := "There are 3 of a max of 20 players online\nTPS from last 1m, 5m, 15m: 19.98, 20.0, 20.0\n"
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	tps, players := observableServerStats(path)
	if tps != "19.98" || players != "3/20" {
		t.Fatalf("got TPS %q players %q", tps, players)
	}
}

func TestStartupCommandPreview(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "paper server.jar"), []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := mcserver.NewManager(dir, "test-", t.TempDir())
	command, err := manager.CommandPreview(dir, mcserver.Settings{JavaPath: "/usr/bin/java", Xms: "1G", Xmx: "2G", ServerJar: "paper server.jar", ServerArgs: []string{"nogui"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "/usr/bin/java -Xms1G -Xmx2G -jar 'paper server.jar' nogui"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}
