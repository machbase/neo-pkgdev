package pkgs

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (r *Roster) Uninstall(name string, output io.Writer, env []string) error {
	meta, err := r.LoadPackageMeta(name)
	if err != nil {
		return err
	}
	inst, err := r.InstalledVersion(name)
	if err != nil {
		return err
	}

	if meta.UninstallRecipe != nil {
		if runtime.GOOS == "windows" {
			// TODO: implement uninstall script for windows
		} else {
			if sc, err := MakeScriptFile(meta.UninstallRecipe.Script, inst.Path, "__uninstall__.sh"); err != nil {
				return err
			} else {
				cmd := exec.Command("sh", "-c", sc)
				cmd.Dir = inst.Path
				cmd.Stdout = output
				cmd.Stderr = output
				cmd.Env = append(os.Environ(), env...)
				err = cmd.Run()
				if err != nil {
					return err
				}
				os.Remove(filepath.Join(inst.Path, "__uninstall__.sh"))
			}
		}
	}

	if !filepath.IsAbs(inst.Path) || !strings.HasPrefix(inst.Path, r.distDir) {
		return fmt.Errorf("invalid installed path: %q", inst.Path)
	}
	if err := os.RemoveAll(inst.Path); err != nil {
		return err
	}
	os.RemoveAll(filepath.Dir(inst.Path))
	return err
}
