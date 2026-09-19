package mcserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetVelocityUpdatesPaperEnabledAndSecret(t *testing.T) {
	serverDir := t.TempDir()
	configDir := filepath.Join(serverDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "paper-global.yml")
	contents := "proxies:\n  velocity:\n    enabled: false\n    online-mode: false\n    secret: ''\nchunk-system:\n  gen-parallelism: default\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SetVelocity(serverDir, true, "proxy-secret"); err != nil {
		t.Fatalf("set velocity: %v", err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "enabled: true") {
		t.Fatalf("enabled setting was not updated:\n%s", updated)
	}
	if !strings.Contains(string(updated), "secret: 'proxy-secret'") {
		t.Fatalf("secret setting was not updated:\n%s", updated)
	}
}

func TestForwardingSecretTrimsWhitespace(t *testing.T) {
	proxyDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(proxyDir, "forwarding.secret"), []byte("  shared-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err := ForwardingSecret(proxyDir)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "shared-secret" {
		t.Fatalf("secret = %q, want shared-secret", secret)
	}
}
