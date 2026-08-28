package httpapi

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mcpanel/internal/mcserver"
	"mcpanel/internal/settings"

	"github.com/gorilla/websocket"
)

type bulkServerRequest struct {
	Names  []string `json:"names"`
	Action string   `json:"action"`
}

func (a *API) BulkServerAction(w http.ResponseWriter, r *http.Request) {
	var req bulkServerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&req); err != nil || len(req.Names) == 0 {
		http.Error(w, "select at least one server", http.StatusBadRequest)
		return
	}
	if len(req.Names) > 100 {
		http.Error(w, "too many servers selected", http.StatusBadRequest)
		return
	}
	if req.Action != "start" && req.Action != "stop" && req.Action != "restart" && req.Action != "backup" && req.Action != "delete" {
		http.Error(w, "unknown bulk action", http.StatusBadRequest)
		return
	}
	results := make(map[string]string, len(req.Names))
	var wg sync.WaitGroup
	var resultsMu sync.Mutex
	for _, name := range req.Names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := "ok"
			if !mcserver.IsKnownServer(a.MCRoot, name, a.ServerSubfolders()) {
				result = "unknown server"
				resultsMu.Lock()
				results[name] = result
				resultsMu.Unlock()
				return
			}
			dir := mcserver.ServerDir(a.MCRoot, name)
			var err error
			switch req.Action {
			case "start":
				err = a.Screen.Start(name, dir, settings.Load(a.DB, name))
			case "stop":
				err = a.Screen.Stop(name, 30*time.Second)
			case "restart":
				err = a.Screen.Restart(name, dir, settings.Load(a.DB, name), 30*time.Second)
			case "backup":
				err = a.backupServer(name)
			case "delete":
				if running, _ := a.Screen.IsRunning(name); running {
					err = fmt.Errorf("stop the server before deleting it")
				} else {
					err = os.RemoveAll(dir)
					if err == nil && a.DB != nil {
						_, _ = a.DB.Exec(`DELETE FROM server_settings WHERE server_name=?`, name)
						a.replaceProjectDeployTarget(name, "")
					}
				}
			}
			if err != nil {
				result = err.Error()
			}
			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}()
	}
	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
}

func (a *API) backupServer(name string) error {
	source := mcserver.ServerDir(a.MCRoot, name)
	backupDir := filepath.Join(a.MCRoot, ".mcpanel-backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	filename := strings.ReplaceAll(name, "/", "_") + "-" + time.Now().Format("20060102-150405") + ".zip"
	file, err := os.Create(filepath.Join(backupDir, filename))
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	walkErr := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name, _ = filepath.Rel(source, path)
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	archiveErr := archive.Close()
	fileErr := file.Close()
	if walkErr != nil {
		return walkErr
	}
	if archiveErr != nil {
		return archiveErr
	}
	return fileErr
}

func (a *API) AllServerStatusesAPI(w http.ResponseWriter, r *http.Request) {
	statuses := a.allServerStatuses()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statuses)
}

func (a *API) allServerStatuses() map[string]string {
	names, _ := mcserver.Discover(a.MCRoot, a.ServerSubfolders())
	statuses := make(map[string]string, len(names))
	for _, name := range names {
		running, _ := a.Screen.IsRunning(name)
		if running {
			statuses[name] = "ONLINE"
		} else {
			statuses[name] = "OFFLINE"
		}
	}
	return statuses
}

func (a *API) ServerStatusesWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	last := ""
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		statuses := a.allServerStatuses()
		encoded, _ := json.Marshal(statuses)
		if string(encoded) != last {
			if err := conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
				return
			}
			last = string(encoded)
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *API) ServerLogDownload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !mcserver.IsKnownServer(a.MCRoot, name, a.ServerSubfolders()) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, strings.ReplaceAll(name, "/", "_")+".log"))
	http.ServeFile(w, r, a.Screen.LogPath(name))
}

func directorySize(root string) uint64 {
	var size uint64
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if info, statErr := entry.Info(); statErr == nil {
				size += uint64(info.Size())
			}
		}
		return nil
	})
	return size
}

type directorySizeEntry struct {
	size     uint64
	measured time.Time
}

var directorySizeCache sync.Map

func cachedDirectorySize(root string) uint64 {
	if cached, ok := directorySizeCache.Load(root); ok {
		entry := cached.(directorySizeEntry)
		if time.Since(entry.measured) < 30*time.Second {
			return entry.size
		}
	}
	size := directorySize(root)
	directorySizeCache.Store(root, directorySizeEntry{size: size, measured: time.Now()})
	return size
}

func copyDirectory(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		inErr := in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if inErr != nil {
			return inErr
		}
		return closeErr
	})
}
