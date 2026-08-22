package mcserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const MaxServerJarBytes int64 = 1024 * 1024 * 1024

var ErrServerExists = errors.New("server directory already exists")

type CreateOptions struct {
	JarURL           string
	JarFilename      string
	AcceptEULA       bool
	ServerProperties map[string]string
	BukkitSettings   map[string]string
}

func DefaultDownloadClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("download redirects must use HTTP or HTTPS")
			}
			return nil
		},
	}
}

// CreateServer assembles a server in a temporary sibling directory and only
// publishes it at targetDir after the JAR and all requested config files have
// been written successfully.
func CreateServer(ctx context.Context, client *http.Client, targetDir string, opts CreateOptions) error {
	if _, err := os.Stat(targetDir); err == nil {
		return ErrServerExists
	} else if !os.IsNotExist(err) {
		return err
	}

	jarURL, err := url.ParseRequestURI(opts.JarURL)
	if err != nil || (jarURL.Scheme != "http" && jarURL.Scheme != "https") || jarURL.Host == "" {
		return errors.New("JAR URL must be a valid HTTP or HTTPS URL")
	}
	jarName := strings.TrimSpace(opts.JarFilename)
	if jarName == "" {
		jarName = "server.jar"
	}
	if filepath.Base(jarName) != jarName || !strings.HasSuffix(strings.ToLower(jarName), ".jar") {
		return errors.New("JAR filename must be a plain filename ending in .jar")
	}

	parent := filepath.Dir(targetDir)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return fmt.Errorf("server parent directory is unavailable")
	}
	tmpDir, err := os.MkdirTemp(parent, "."+filepath.Base(targetDir)+"-creating-")
	if err != nil {
		return err
	}
	defer func() {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if client == nil {
		client = DefaultDownloadClient()
	}
	if err := downloadJar(ctx, client, opts.JarURL, filepath.Join(tmpDir, jarName)); err != nil {
		return err
	}
	if opts.AcceptEULA {
		if err := os.WriteFile(filepath.Join(tmpDir, "eula.txt"), []byte("eula=true\n"), 0o644); err != nil {
			return err
		}
	}
	if len(opts.ServerProperties) > 0 {
		content, err := renderProperties(opts.ServerProperties)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "server.properties"), content, 0o644); err != nil {
			return err
		}
	}
	if len(opts.BukkitSettings) > 0 {
		content, err := renderBukkitYAML(opts.BukkitSettings)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "bukkit.yml"), content, 0o644); err != nil {
			return err
		}
	}

	if err := os.Rename(tmpDir, targetDir); err != nil {
		return err
	}
	tmpDir = ""
	return nil
}

func downloadJar(ctx context.Context, client *http.Client, sourceURL, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download server JAR: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download server JAR: remote server returned %s", resp.Status)
	}
	if resp.ContentLength > MaxServerJarBytes {
		return errors.New("download server JAR: file exceeds 1 GiB limit")
	}

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(out, io.LimitReader(resp.Body, MaxServerJarBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("download server JAR: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if written > MaxServerJarBytes {
		return errors.New("download server JAR: file exceeds 1 GiB limit")
	}

	f, err := os.Open(destination)
	if err != nil {
		return err
	}
	var magic [2]byte
	_, readErr := io.ReadFull(f, magic[:])
	_ = f.Close()
	if readErr != nil || string(magic[:]) != "PK" {
		return errors.New("downloaded file is not a valid JAR/ZIP file")
	}
	return nil
}

func renderProperties(values map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(values))
	normalized := make(map[string]string, len(values))
	for rawKey, value := range values {
		key := strings.TrimSpace(rawKey)
		if !validConfigKey(key) {
			return nil, fmt.Errorf("invalid server.properties key %q", rawKey)
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("server.properties value for %q must be one line", key)
		}
		if _, exists := normalized[key]; exists {
			return nil, fmt.Errorf("duplicate server.properties key %q", key)
		}
		normalized[key] = value
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# Generated by MCPanel before first launch\n")
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(normalized[key])
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func renderBukkitYAML(values map[string]string) ([]byte, error) {
	root := map[string]any{}
	for dottedKey, rawValue := range values {
		dottedKey = strings.TrimSpace(dottedKey)
		if !validConfigKey(dottedKey) {
			return nil, fmt.Errorf("invalid bukkit.yml path %q", dottedKey)
		}
		parts := strings.Split(dottedKey, ".")
		current := root
		for _, part := range parts[:len(parts)-1] {
			next, exists := current[part]
			if !exists {
				child := map[string]any{}
				current[part] = child
				current = child
				continue
			}
			child, ok := next.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("conflicting bukkit.yml path %q", dottedKey)
			}
			current = child
		}
		leaf := parts[len(parts)-1]
		if _, exists := current[leaf]; exists {
			return nil, fmt.Errorf("duplicate bukkit.yml path %q", dottedKey)
		}
		value, err := parseYAMLScalar(rawValue)
		if err != nil {
			return nil, fmt.Errorf("invalid bukkit.yml value for %q: %w", dottedKey, err)
		}
		current[leaf] = value
	}
	content, err := yaml.Marshal(root)
	if err != nil {
		return nil, err
	}
	return append([]byte("# Generated by MCPanel before first launch\n"), content...), nil
}

func parseYAMLScalar(raw string) (any, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		return nil, err
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.ScalarNode {
		return nil, errors.New("value must be a YAML scalar")
	}
	var value any
	if err := node.Content[0].Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func validConfigKey(key string) bool {
	if key == "" || strings.HasPrefix(key, ".") || strings.HasSuffix(key, ".") || strings.Contains(key, "..") {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}
