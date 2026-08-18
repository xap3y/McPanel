package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrOutsideRoot = errors.New("path escapes configured root")

// SafeJoin resolves userPath relative to root and guarantees the
// result is inside root, following symlinks. Works for paths that
// don't exist yet (e.g. creating a new file) by walking up to the
// nearest existing ancestor for the symlink check.
func SafeJoin(root, userPath string) (string, error) {
	root = filepath.Clean(root)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}

	joined := filepath.Join(root, userPath)
	joined = filepath.Clean(joined)

	if !isWithin(realRoot, joined) {
		return "", ErrOutsideRoot
	}

	// Resolve symlinks on the deepest existing ancestor to catch
	// symlink traversal even for not-yet-existing target files.
	existing := joined
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		existing = parent
	}
	realExisting, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	if !isWithin(realRoot, realExisting) {
		return "", ErrOutsideRoot
	}

	return joined, nil
}

func isWithin(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	return strings.HasPrefix(target, root+string(filepath.Separator))
}