package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListArtifactsReturnsEmptySlice(t *testing.T) {
	artifacts, err := ListArtifacts(t.TempDir())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if artifacts == nil || len(artifacts) != 0 {
		t.Fatalf("expected a non-nil empty slice, got %#v", artifacts)
	}
}

func TestListArtifactsReturnsEveryDeployableJar(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"build/libs/one.jar", "build/libs/two.jar", "target/three.jar", "deploy/four.jar", "build/libs/four-sources.jar"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("jar"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ListArtifacts(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"build/libs/one.jar", "build/libs/two.jar", "deploy/four.jar", "target/three.jar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifacts = %#v, want %#v", got, want)
	}
}
