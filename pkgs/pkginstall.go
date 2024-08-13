package pkgs

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/machbase/neo-pkgdev/pkgs/untar"
)

type InstallStatus struct {
	PkgName   string            `json:"pkg_name"`
	Err       error             `json:"error,omitempty"`
	Installed *InstalledVersion `json:"installed,omitempty"`
	Output    string            `json:"output,omitempty"`
}

func (r *Roster) Install(pkgs []string, env []string) []*InstallStatus {
	ret := make([]*InstallStatus, len(pkgs))
	for i, name := range pkgs {
		output := &strings.Builder{}
		if err := r.install0(name, output, env); err != nil {
			ret[i] = &InstallStatus{
				PkgName: name,
				Err:     err,
				Output:  output.String(),
			}
		} else {
			inst, err := r.InstalledVersion(name)
			ret[i] = &InstallStatus{
				PkgName:   name,
				Err:       err,
				Installed: inst,
				Output:    output.String(),
			}
		}
	}
	return ret
}

// Install installs the package to the distDir
// returns the installed symlink path '~/dist/<name>/current'
func (r *Roster) install0(name string, output io.Writer, env []string) error {
	meta, err := r.LoadPackageMeta(name)
	if err != nil {
		return err
	}
	cache, err := r.LoadPackageCache(name)
	if err != nil {
		return err
	}

	force := true
	dist, _ := cache.RemoteDistribution()
	thisPkgDir := filepath.Join(r.distDir, cache.Name)
	archiveFile := filepath.Join(thisPkgDir, dist.ArchiveBase)
	unarchiveDir := filepath.Join(thisPkgDir, dist.UnarchiveDir)
	currentVerDir := filepath.Join(thisPkgDir, "current")

	if err := os.MkdirAll(unarchiveDir, 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	if _, err := os.Stat(archiveFile); err == nil && !force {
		return fmt.Errorf("file %q already exists", archiveFile)
	}

	var srcUrl *url.URL
	if cache.Url != "" {
		if u, err := url.Parse(cache.Url); err != nil {
			return err
		} else {
			srcUrl = u
		}
	} else {
		if u, err := url.Parse(dist.Url); err != nil {
			return err
		} else {
			srcUrl = u
		}
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		// Timeout: time.Duration(10) * time.Second, // download takes longer than 10 seconds
	}

	rsp, err := httpClient.Do(&http.Request{
		Method: "GET",
		URL:    srcUrl,
	})
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		content, _ := io.ReadAll(rsp.Body)
		return fmt.Errorf("failed to download %q: %s %s", srcUrl, rsp.Status, string(content))
	}

	download, err := os.OpenFile(archiveFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	_, err = io.Copy(download, rsp.Body)
	if err != nil {
		return err
	}
	download.Close()

	switch strings.ToLower(dist.ArchiveExt) {
	case ".zip":
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell", "-Command", "Expand-Archive", "-Path", archiveFile, "-DestinationPath", unarchiveDir)
		} else {
			cmd = exec.Command("unzip", "-o", "-d", unarchiveDir, archiveFile)
		}
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
			return err
		}
	case ".tar.gz":
		fd, err := os.Open(archiveFile)
		if err != nil {
			return err
		}
		defer fd.Close()
		if err := untar.Untar(fd, unarchiveDir, dist.StripComponents); err != nil {
			return err
		}
	}
	inst, err := r.InstalledVersion(name)
	if _, err := os.Stat(currentVerDir); err == nil {
		// remove symlink
		if err := os.Remove(currentVerDir); err != nil {
			return err
		}
	}
	if err == nil && inst != nil && inst.Path != "" {
		// remove old version
		os.RemoveAll(inst.Path)
	}
	// new symlink
	err = os.Symlink(unarchiveDir, currentVerDir)
	if err != nil {
		return err
	}

	if meta.InstallRecipe != nil {
		// TODO: make install recipe to work on windows
		if runtime.GOOS == "windows" {

		} else {
			if sc, err := MakeScriptFile(meta.InstallRecipe.Script, unarchiveDir, "__install__.sh"); err != nil {
				return err
			} else {
				cmd := exec.Command("sh", "-c", sc)
				cmd.Dir = currentVerDir
				cmd.Stdout = output
				cmd.Stderr = output
				cmd.Env = append(os.Environ(), env...)
				err = cmd.Run()
				if err != nil {
					return err
				}
			}
			os.Remove(filepath.Join(unarchiveDir, "__install__.sh"))
		}
	}
	// remove archive file
	os.Remove(archiveFile)
	return nil
}
