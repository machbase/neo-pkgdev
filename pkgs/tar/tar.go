package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
		if stat.IsDir() {
			if err := tarDir(tw, file, stat); err != nil {
				return err
			}
		} else {
			if err := tarFile(tw, file, stat); err != nil {
				return err
			}
		}
	}
	return nil
}

func tarDir(tw *tar.Writer, path string, fi os.FileInfo) error {
	mode := fi.Mode()
	modTime := fi.ModTime()
	uid, gid := getUid(fi)
	hdr := &tar.Header{
		Typeflag: byte(tar.TypeDir),
		Name:     filepath.ToSlash(path),
		Mode:     int64(mode.Perm()),
		ModTime:  modTime,
		Uid:      uid,
		Gid:      gid,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		log.Fatal(err)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Println("readDir", path)
		return err
	}
	for _, entry := range entries {
		name := filepath.Join(path, entry.Name())
		stat, err := os.Stat(name)
		if err != nil {
			fmt.Println("stat", name)
			return err
		}
		if stat.IsDir() {
			if err := tarDir(tw, name+string(filepath.Separator), stat); err != nil {
				return err
			}
		} else {
			if err := tarFile(tw, name, stat); err != nil {
				return err
			}
		}
	}
	return nil
}

func tarFile(tw *tar.Writer, path string, fi os.FileInfo) error {
	size := fi.Size()
	mode := fi.Mode()
	modTime := fi.ModTime()
	uid, gid := getUid(fi)
	hdr := &tar.Header{
		Typeflag: byte(tar.TypeReg),
		Name:     filepath.ToSlash(path),
		Mode:     int64(mode.Perm()),
		Size:     size,
		ModTime:  modTime,
		Uid:      uid,
		Gid:      gid,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		log.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(tw, f); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}
