//go:build !windows
// +build !windows

package tar

import (
	"io/fs"
	"os"
	"syscall"
)

func getUid(stat fs.FileInfo) (int, int) {
	var uid, gid int
	if sys_stat, ok := stat.Sys().(*syscall.Stat_t); ok {
		uid = int(sys_stat.Uid)
		gid = int(sys_stat.Gid)
	} else {
		// not in linux
		uid = os.Getuid()
		gid = os.Getgid()
	}
	return uid, gid
}
