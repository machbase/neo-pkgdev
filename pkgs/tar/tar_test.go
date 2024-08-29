package tar_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/machbase/neo-pkgdev/pkgs/tar"
	"github.com/machbase/neo-pkgdev/pkgs/untar"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	provides := []string{
		"build/",
	}
	err := tar.Archive("testdata", "test.tar.gz", provides)
	if err != nil {
		panic(err)
	}

	fd, err := os.Open("testdata/test.tar.gz")
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	if err := untar.Untar(fd, "testdata/extract", 1); err != nil {
		panic(err)
	}

	buf := &strings.Builder{}
	cmd := exec.Command("tar", "tf", "testdata/test.tar.gz")
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Run()

	expects := []string{
		"build/",
		"build/subdir/",
		"build/subdir/hello.txt",
		"build/test.txt",
		"",
	}
	result := strings.Split(buf.String(), "\n")
	for i := range expects {
		result[i] = strings.TrimSpace(result[i])
	}
	require.Equal(t, expects, result)

	err = os.Remove("testdata/test.tar.gz")
	if err != nil {
		panic(err)
	}

}
