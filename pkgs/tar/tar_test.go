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
		"build/",
	}
	err := tar.Archive("testdata", "test.tar.gz", provides)
	if err != nil {
		panic(err)
	}
	buf := &strings.Builder{}
	cmd := exec.Command("tar", "tf", "testdata/test.tar.gz")
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Run()

	err = os.Remove("testdata/test.tar.gz")
	if err != nil {
		panic(err)
	}

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
}
