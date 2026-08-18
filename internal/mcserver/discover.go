package mcserver

import (
	"os"
	"path/filepath"
)

// Discover lists every immediate subdirectory of root as a candidate
// server. This is the single source of truth for "valid server names" -
// nothing downstream should accept a server name that didn't come from here.
func Discover(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name()[0] == '.' {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}

func IsKnownServer(root, name string) bool {
	names, err := Discover(root)
	if err != nil {
		return false
	}
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func ServerDir(root, name string) string {
	return filepath.Join(root, name)
}