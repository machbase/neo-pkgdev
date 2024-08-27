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
	distAvailable, _ := cache.RemoteDistribution()
	var dist *PackageDistribution
	for _, d := range distAvailable {
		if d.PlatformOS == "" && d.PlatformArch == "" {
			dist = d
			break
		}
		if d.PlatformOS == runtime.GOOS && d.PlatformArch == runtime.GOARCH {
			dist = d
			break
		}
	}
	if dist == nil {
		return fmt.Errorf("no distribution for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	thisPkgDir := filepath.Join(r.distDir, cache.Name)
	archiveFile := filepath.Join(thisPkgDir, dist.ArchiveBase)
	unarchiveDir := filepath.Join(thisPkgDir, dist.UnarchiveDir)
	currentVerDir := filepath.Join(thisPkgDir, "current")
	wip := filepath.Join(thisPkgDir, "wip") // work in progress

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

	os.WriteFile(wip, []byte(dist.Url), 0644)
	defer func() {
		os.Remove(wip)
	}()

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
		if err := untar.Untar(fd, unarchiveDir, dist.StripComponents); err != nil {
			fd.Close()
			return err
		}
		fd.Close()
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
	// !! windows requires abs path
	oldName, _ := filepath.Abs(filepath.FromSlash(unarchiveDir))
	newName, _ := filepath.Abs(filepath.FromSlash(currentVerDir))
	err = Symlink(oldName, newName)
	if err != nil {
		return fmt.Errorf("symlink %q -> %q: %w", oldName, newName, err)
	}

	installRun := FindScript(meta.InstallRecipe.Scripts, runtime.GOOS)
	if runtime.GOOS == "windows" {
		if sc, err := MakeScriptFile([]string{installRun}, unarchiveDir, "__install__.cmd"); err != nil {
			r.log.Errorf("make script file: %v", err)
			return err
		} else {
			cmd := exec.Command("cmd", "/c", sc)
			cmd.Dir = unarchiveDir
			cmd.Stdout = output
			cmd.Stderr = output
			cmd.Env = append(os.Environ(), env...)
			err = cmd.Run()
			if err != nil {
				r.log.Warnf("running install script %q: %v", sc, err)
				return err
			}
		}
		os.Remove(filepath.Join(unarchiveDir, "__install__.cmd"))
	} else {
		if meta.InstallRecipe != nil {
			if sc, err := MakeScriptFile([]string{installRun}, unarchiveDir, "__install__.sh"); err != nil {
				return err
			} else {
				cmd := exec.Command("sh", "-c", sc)
				cmd.Dir = unarchiveDir
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
	err = os.Remove(archiveFile)
	if err != nil {
		r.log.Errorf("cleaning download file %q: %v", archiveFile, err)
	}
	return nil
}
