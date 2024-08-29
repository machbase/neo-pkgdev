//go:build windows
// +build windows

package tar

import (
	"io/fs"
	"os"
)

func getUid(_ fs.FileInfo) (int, int) {
	uid := os.Getuid()
	gid := os.Getgid()
	return uid, gid
}
