//go:build !windows
// +build !windows

package pkgs

import "os"

func Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func Symlink(oldName, newName string) error {
	return os.Symlink(oldName, newName)
}
