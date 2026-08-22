package mcserver

import (
	"path/filepath"
	"testing"
)

func TestRuntimePathsFlattenGroupedServerName(t *testing.T) {
	m := NewManager("/servers", "mcpanel-", t.TempDir())
	if got, want := filepath.Base(m.LogPath("folia/1.21.11")), "folia_1.21.11.log"; got != want {
		t.Fatalf("LogPath() = %q, want %q", got, want)
	}
	if got, want := filepath.Base(m.screenRCPath("folia/1.21.11")), "folia_1.21.11.screenrc"; got != want {
		t.Fatalf("screenRCPath() = %q, want %q", got, want)
	}
}
