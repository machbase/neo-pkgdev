package tar

import (
	"archive/tar"
	"io"
	"log"
	"os"
)

func Archive(dest string, files []string) error {
	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer destFile.Close()
	tw := tar.NewWriter(destFile)
	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			return err
		}
		size := stat.Size()
		mode := stat.Mode()
		modTime := stat.ModTime()
		var fileType byte
		if stat.IsDir() {
			fileType = tar.TypeDir
		} else {
			fileType = tar.TypeReg
		}
		uid, gid := getUid(stat)
		hdr := &tar.Header{
			Typeflag: fileType,
			Name:     file,
			Mode:     int64(mode.Perm()),
			Size:     size,
			ModTime:  modTime,
			Uid:      uid,
			Gid:      gid,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatal(err)
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		log.Fatal(err)
	}
	return nil
}
