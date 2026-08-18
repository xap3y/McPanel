package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DetectArtifact finds the most recently built jar under build/libs,
// excluding -sources.jar / -javadoc.jar.
func DetectArtifact(projectDir, glob string) (string, error) {
	artifacts, err := listArtifactPaths(projectDir, glob)
	if err != nil {
		return "", err
	}
	type cand struct {
		path string
		mod  int64
	}
	var candidates []cand
	for _, path := range artifacts {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		candidates = append(candidates, cand{path: path, mod: info.ModTime().Unix()})
	}
	if len(candidates) == 0 {
		return "", os.ErrNotExist
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].mod > candidates[j].mod })
	return candidates[0].path, nil
}

func ListArtifacts(projectDir string) ([]string, error) {
	paths, err := listArtifactPaths(projectDir, "*.jar")
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if rel, err := filepath.Rel(projectDir, path); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
	}
	sort.Strings(out)
	return out, nil
}

// listArtifactPaths covers Gradle, Maven, and a project's deploy directory.
func listArtifactPaths(projectDir, glob string) ([]string, error) {
	dirs := []string{filepath.Join(projectDir, "build", "libs"), filepath.Join(projectDir, "target"), filepath.Join(projectDir, "deploy")}
	var out []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".jar") || strings.HasSuffix(name, "-sources.jar") || strings.HasSuffix(name, "-javadoc.jar") {
				continue
			}
			if glob != "" && glob != "*.jar" {
				if ok, _ := filepath.Match(glob, name); !ok {
					continue
				}
			}
			out = append(out, filepath.Join(dir, name))
		}
	}
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	return out, nil
}
