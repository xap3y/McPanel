package mcserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PluginInfo struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func ListPlugins(serverDir string) ([]PluginInfo, error) {
	dir := filepath.Join(serverDir, "plugins")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []PluginInfo{}, nil
		}
		return nil, err
	}
	out := make([]PluginInfo, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".jar.d"):
			out = append(out, PluginInfo{Name: strings.TrimSuffix(name, ".d"), Enabled: false})
		case strings.HasSuffix(name, ".jar"):
			out = append(out, PluginInfo{Name: name, Enabled: true})
		}
	}
	return out, nil
}

func TogglePlugin(serverDir, name string, enable bool) error {
	dir := filepath.Join(serverDir, "plugins")
	var from, to string
	if enable {
		from, to = filepath.Join(dir, name+".d"), filepath.Join(dir, name)
	} else {
		from, to = filepath.Join(dir, name), filepath.Join(dir, name+".d")
	}
	if _, err := os.Stat(to); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(to))
	}
	return os.Rename(from, to)
}

func DeletePlugin(serverDir, name string, enabled bool) error {
	if filepath.Base(name) != name || !strings.HasSuffix(strings.ToLower(name), ".jar") {
		return fmt.Errorf("invalid plugin name")
	}
	if !enabled {
		name += ".d"
	}
	return os.Remove(filepath.Join(serverDir, "plugins", name))
}
