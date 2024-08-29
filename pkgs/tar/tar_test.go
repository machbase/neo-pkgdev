package tar_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/machbase/neo-pkgdev/pkgs/tar"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	provides := []string{
		"testdata/build/",
	}
	err := tar.Archive("test.tar.gz", provides)
	if err != nil {
		panic(err)
	}
	buf := &strings.Builder{}
	cmd := exec.Command("tar", "tf", "test.tar.gz")
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Run()

	err = os.Remove("test.tar.gz")
	if err != nil {
		panic(err)
	}

	expects := []string{
		"testdata/build/",
		"testdata/build/subdir/",
		"testdata/build/subdir/hello.txt",
		"testdata/build/test.txt",
		"",
	}
	result := strings.Split(buf.String(), "\n")
	for i := range expects {
		result[i] = strings.TrimSpace(result[i])
	}
	require.Equal(t, expects, result)
}
