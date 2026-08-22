package mcserver

import (
	"os"
	"path/filepath"
)

func Discover(root string, subfolders []string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	isSubfolder := func(name string) bool {
		for _, sf := range subfolders {
			if sf == name {
				return true
			}
		}
		return false
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name()[0] == '.' {
			continue
		}

		if isSubfolder(e.Name()) {
			subEntries, err := os.ReadDir(filepath.Join(root, e.Name()))
			if err == nil {
				for _, se := range subEntries {
					if !se.IsDir() || se.Name()[0] == '.' {
						continue
					}
					names = append(names, e.Name() + "/" + se.Name())
				}
			}
		} else {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func IsKnownServer(root, name string, subfolders []string) bool {
	names, err := Discover(root, subfolders)
	if err != nil {
		return false
	}
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func ServerDir(root, name string) string {
	return filepath.Join(root, name)
}
