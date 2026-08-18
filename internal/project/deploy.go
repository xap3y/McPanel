package project

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Deploy(db *sql.DB, projectName, serverName, serverDir, artifactPath, destName string, buildID int64) error {
	pluginsDir := filepath.Join(serverDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return record(db, projectName, serverName, buildID, "FAILED", err.Error())
	}

	tmp := filepath.Join(pluginsDir, "."+destName+".tmp")
	final := filepath.Join(pluginsDir, destName)

	if err := copyFile(artifactPath, tmp); err != nil {
		return record(db, projectName, serverName, buildID, "FAILED", err.Error())
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return record(db, projectName, serverName, buildID, "FAILED", err.Error())
	}

	removeStaleVersions(pluginsDir, destName)
	return record(db, projectName, serverName, buildID, "SUCCESS", "")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// removeStaleVersions deletes older Name-1.2.0.jar style files once
// Name.jar has been deployed, so two versions never load together.
func removeStaleVersions(pluginsDir, destName string) {
	base := strings.TrimSuffix(destName, ".jar")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == destName || !strings.HasSuffix(name, ".jar") {
			continue
		}
		if strings.HasPrefix(name, base+"-") {
			os.Remove(filepath.Join(pluginsDir, name))
		}
	}
}

func record(db *sql.DB, projectName, serverName string, buildID int64, status, errMsg string) error {
	var bid interface{}
	if buildID > 0 {
		bid = buildID
	}
	var errNull interface{}
	if errMsg != "" {
		errNull = errMsg
	}
	db.Exec(`INSERT INTO deploy_history (project_name, server_name, build_id, status, error_message) VALUES (?, ?, ?, ?, ?)`,
		projectName, serverName, bid, status, errNull)
	if status == "FAILED" {
		return fmt.Errorf(errMsg)
	}
	return nil
}