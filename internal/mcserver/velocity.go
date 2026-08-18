package mcserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SetVelocity(serverDir string, enabled bool) error {
	modified := false

	// 1. Try paper-global.yml (Paper 1.19+)
	pgPath := filepath.Join(serverDir, "config", "paper-global.yml")
	if data, err := os.ReadFile(pgPath); err == nil {
		lines := strings.Split(string(data), "\n")
		inProxies, inVelocity := false, false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			indent := len(line) - len(strings.TrimLeft(line, " "))

			if trimmed == "proxies:" && indent == 0 {
				inProxies = true
				continue
			}
			if inProxies && indent == 0 && trimmed != "" && trimmed != "proxies:" {
				inProxies = false
			}
			if inProxies && trimmed == "velocity:" && indent == 2 {
				inVelocity = true
				continue
			}
			if inVelocity && indent <= 2 && trimmed != "" {
				inVelocity = false
			}
			if inVelocity && strings.HasPrefix(trimmed, "enabled:") {
				prefix := line[:indent]
				lines[i] = fmt.Sprintf("%senabled: %t", prefix, enabled)
				modified = true
				break
			}
		}
		if modified {
			_ = os.WriteFile(pgPath, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}

	// 2. Try paper.yml (Paper 1.13 - 1.18)
	if !modified {
		pPath := filepath.Join(serverDir, "paper.yml")
		if data, err := os.ReadFile(pPath); err == nil {
			lines := strings.Split(string(data), "\n")
			inSettings, inVelocity := false, false
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				indent := len(line) - len(strings.TrimLeft(line, " "))

				if trimmed == "settings:" && indent == 0 {
					inSettings = true
					continue
				}
				if inSettings && indent == 0 && trimmed != "" && trimmed != "settings:" {
					inSettings = false
				}
				if inSettings && trimmed == "velocity-support:" && indent == 2 {
					inVelocity = true
					continue
				}
				if inVelocity && indent <= 2 && trimmed != "" {
					inVelocity = false
				}
				if inVelocity && strings.HasPrefix(trimmed, "enabled:") {
					prefix := line[:indent]
					lines[i] = fmt.Sprintf("%senabled: %t", prefix, enabled)
					modified = true
					break
				}
			}
			if modified {
				_ = os.WriteFile(pPath, []byte(strings.Join(lines, "\n")), 0o644)
			}
		}
	}

	// 3. Try spigot.yml (Legacy 1.12.2 and older / BungeeCord mode)
	if !modified {
		sPath := filepath.Join(serverDir, "spigot.yml")
		if data, err := os.ReadFile(sPath); err == nil {
			lines := strings.Split(string(data), "\n")
			inSettings := false
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				indent := len(line) - len(strings.TrimLeft(line, " "))

				if trimmed == "settings:" && indent == 0 {
					inSettings = true
					continue
				}
				if inSettings && indent == 0 && trimmed != "" && trimmed != "settings:" {
					inSettings = false
				}
				if inSettings && strings.HasPrefix(trimmed, "bungeecord:") {
					prefix := line[:indent]
					lines[i] = fmt.Sprintf("%sbungeecord: %t", prefix, enabled)
					modified = true
					break
				}
			}
			if modified {
				_ = os.WriteFile(sPath, []byte(strings.Join(lines, "\n")), 0o644)
			}
		}
	}

	if !modified {
		return fmt.Errorf("velocity or bungeecord setting not found (has the server been started at least once?)")
	}

	return nil
}