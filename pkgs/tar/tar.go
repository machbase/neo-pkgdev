package tar

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Archive(cwd string, dest string, files []string) error {
	destFile, err := os.OpenFile(filepath.Join(cwd, dest), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer destFile.Close()

	var tw *tar.Writer
	if strings.HasSuffix(dest, ".tar.gz") || strings.HasSuffix(dest, ".tgz") {
		compressor, err := gzip.NewWriterLevel(destFile, gzip.BestCompression)
		if err != nil {
			return err
		}
		defer compressor.Close()
		tw = tar.NewWriter(compressor)
	} else {
		tw = tar.NewWriter(destFile)
	}
	for _, file := range files {
		stat, err := os.Stat(filepath.Join(cwd, file))
		if err != nil {
			return err
		}
		if stat.IsDir() {
			if err := tarDir(tw, cwd, file, stat); err != nil {
				return err
			}
		} else {
			if err := tarFile(tw, cwd, file, stat); err != nil {
				return err
			}
		}
	}
	tw.Close()
	return nil
}

func tarDir(tw *tar.Writer, cwd string, path string, fi os.FileInfo) error {
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
	entries, err := os.ReadDir(filepath.Join(cwd, path))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := filepath.Join(path, entry.Name())
		stat, err := os.Stat(filepath.Join(cwd, name))
		if err != nil {
			return err
		}
		if stat.IsDir() {
			if err := tarDir(tw, cwd, name+string(filepath.Separator), stat); err != nil {
				return err
			}
		} else {
			if err := tarFile(tw, cwd, name, stat); err != nil {
				return err
			}
		}
	}
	return nil
}

func tarFile(tw *tar.Writer, cwd string, path string, fi os.FileInfo) error {
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
	f, err := os.Open(filepath.Join(cwd, path))
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
