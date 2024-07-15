package utils

import (
	"os"
	"path/filepath"
)

// GetFullPath takes a relative path and returns the full absolute path
func GetFullPath(path string) (string, error) {
	cleanedPath := filepath.Clean(path)
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(currentDir, cleanedPath)
	return fullPath, nil
}
