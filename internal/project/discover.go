package project

import (
	"os"
	"path/filepath"
)

type Project struct {
	Name string
	Dir  string
}

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

func ProjectDir(root, name string) string { return filepath.Join(root, name) }