package project

import (
	"database/sql"
	"os"
	"path/filepath"
)

type Project struct {
	Name   string
	Dir    string
	Custom bool // true if added manually (not auto-discovered from root)
}

// Discover auto-scans the root directory for Gradle projects.
func Discover(root string) ([]Project, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Project
	for _, e := range entries {
		if !e.IsDir() || e.Name()[0] == '.' {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if looksLikeGradleProject(dir) {
			out = append(out, Project{Name: e.Name(), Dir: dir})
		}
	}
	return out, nil
}

// DiscoverAll returns auto-discovered projects merged with custom ones from DB.
// Custom projects that shadow a name already in root are skipped.
func DiscoverAll(root string, db *sql.DB) ([]Project, error) {
	projects, err := Discover(root)
	if err != nil {
		// Don't fail completely if root is inaccessible
		projects = []Project{}
	}

	if db == nil {
		return projects, nil
	}

	// Build a name-set of already-found projects
	existing := make(map[string]bool)
	for _, p := range projects {
		existing[p.Name] = true
	}

	rows, err := db.Query(`SELECT name, dir FROM custom_projects ORDER BY name`)
	if err != nil {
		return projects, nil // Table may not exist yet
	}
	defer rows.Close()
	for rows.Next() {
		var name, dir string
		if err := rows.Scan(&name, &dir); err != nil {
			continue
		}
		if existing[name] {
			continue // auto-discovered wins
		}
		projects = append(projects, Project{Name: name, Dir: dir, Custom: true})
	}
	return projects, nil
}

// AddCustomProject registers a project with a custom directory in the DB.
func AddCustomProject(db *sql.DB, name, dir string) error {
	_, err := db.Exec(`
		INSERT INTO custom_projects (name, dir) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE dir=VALUES(dir)`, name, dir)
	return err
}

// RemoveCustomProject removes a custom project from the DB.
func RemoveCustomProject(db *sql.DB, name string) error {
	_, err := db.Exec(`DELETE FROM custom_projects WHERE name=?`, name)
	return err
}

func looksLikeGradleProject(dir string) bool {
	for _, c := range []string{"gradlew", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			return true
		}
	}
	return false
}

func IsKnownProject(root, name string) bool {
	projects, err := Discover(root)
	if err != nil {
		return false
	}
	for _, p := range projects {
		if p.Name == name {
			return true
		}
	}
	return false
}

// IsKnownProjectDB checks both auto-discovered and custom projects.
func IsKnownProjectDB(root, name string, db *sql.DB) bool {
	projects, _ := DiscoverAll(root, db)
	for _, p := range projects {
		if p.Name == name {
			return true
		}
	}
	return false
}

// ProjectDirDB returns the directory for a project, checking custom ones too.
func ProjectDirDB(root, name string, db *sql.DB) string {
	projects, _ := DiscoverAll(root, db)
	for _, p := range projects {
		if p.Name == name {
			return p.Dir
		}
	}
	return filepath.Join(root, name) // fallback
}

func ProjectDir(root, name string) string { return filepath.Join(root, name) }