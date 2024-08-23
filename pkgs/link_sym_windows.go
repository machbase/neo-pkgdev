//go:build windows
// +build windows

package pkgs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func Readlink(name string) (string, error) {
	content, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	if ret := strings.TrimSpace(string(content)); ret == "" {
		return "", errors.New("not a symlink")
	} else {
		return ret, nil
	}
}

func Symlink(oldName, newName string) error {
	oldName = filepath.Clean(oldName)
	return os.WriteFile(newName, []byte(oldName), 0644)
}
