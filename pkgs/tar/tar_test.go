package tar_test

import (
	"os"
	"os/exec"

	"github.com/machbase/neo-pkgdev/pkgs/tar"
)

func ExampleArchive() {
	provides := []string{
		"testdata/",
	}
	err := tar.Archive("test.tar.gz", provides)
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("tar", "tf", "test.tar.gz")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	err = os.Remove("test.tar.gz")
	if err != nil {
		panic(err)
	}

	// Output:
	// testdata/
	// testdata/subdir/
	// testdata/subdir/hello.txt
	// testdata/test.txt

}
