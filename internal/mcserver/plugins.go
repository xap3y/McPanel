package mcserver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PluginInfo struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	SizeBytes int64  `json:"sizeBytes"`
}

// UploadPlugin stores a new enabled plugin JAR. Plugin uploads never replace
// an existing file, so an accidental upload cannot overwrite a plugin in use.
func UploadPlugin(serverDir, name string, source io.Reader) error {
	if filepath.Base(name) != name || !strings.HasSuffix(strings.ToLower(name), ".jar") {
		return fmt.Errorf("plugin must be a .jar file")
	}

	dir := filepath.Join(serverDir, "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	destination := filepath.Join(dir, name)
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("%s already exists", name)
	} else if !os.IsNotExist(err) {
		return err
	}

	temporary, err := os.CreateTemp(dir, ".plugin-upload-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := io.Copy(temporary, source); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destination)
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
		var size int64
		if info, infoErr := e.Info(); infoErr == nil {
			size = info.Size()
		}
		switch {
		case strings.HasSuffix(name, ".jar.d"):
			out = append(out, PluginInfo{Name: strings.TrimSuffix(name, ".d"), Enabled: false, SizeBytes: size})
		case strings.HasSuffix(name, ".jar"):
			out = append(out, PluginInfo{Name: name, Enabled: true, SizeBytes: size})
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

// CopyPlugin copies a plugin JAR to another server while preserving whether it
// is enabled. Existing destination plugins are never overwritten.
func CopyPlugin(sourceDir, destinationDir, name string, enabled bool) error {
	if filepath.Base(name) != name || !strings.HasSuffix(strings.ToLower(name), ".jar") {
		return fmt.Errorf("invalid plugin name")
	}
	if !enabled {
		name += ".d"
	}

	sourcePath := filepath.Join(sourceDir, "plugins", name)
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("plugin is not a regular file")
	}

	destinationPlugins := filepath.Join(destinationDir, "plugins")
	if err := os.MkdirAll(destinationPlugins, 0755); err != nil {
		return err
	}
	destinationPath := filepath.Join(destinationPlugins, name)
	if _, err := os.Stat(destinationPath); err == nil {
		return fmt.Errorf("%s already exists on destination server", name)
	} else if !os.IsNotExist(err) {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	temporary, err := os.CreateTemp(destinationPlugins, ".plugin-copy-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := io.Copy(temporary, source); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destinationPath)
}
