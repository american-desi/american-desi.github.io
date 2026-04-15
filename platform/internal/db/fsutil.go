package db

import (
	"os"
	"path/filepath"
)

func ensureParentDir(p string) error {
	dir := filepath.Dir(p)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func readFile(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
