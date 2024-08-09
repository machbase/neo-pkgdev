package pkgs

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type RosterName string

const ROSTER_CENTRAL RosterName = "central"

var ROSTER_REPOS = map[RosterName]string{
	ROSTER_CENTRAL: "https://github.com/machbase/neo-pkg.git",
}

type Roster struct {
	metaDir string
	distDir string
}

type RosterOption func(*Roster)

func NewRoster(baseDir string, opts ...RosterOption) (*Roster, error) {
	if abs, err := filepath.Abs(baseDir); err != nil {
		return nil, err
	} else {
		baseDir = abs
	}
	metaDir := filepath.Join(baseDir, "meta")
	distDir := filepath.Join(baseDir, "dist")

	ret := &Roster{
		metaDir: metaDir,
		distDir: distDir,
	}
	for _, opt := range opts {
		opt(ret)
	}
	for _, dir := range []string{metaDir, distDir} {
		if _, err := os.Stat(dir); err != nil {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}
	}
	return ret, nil
}

type Updates struct {
	Upgradable []*Upgradable `json:"upgradable"`
}

type Upgradable struct {
	PkgName          string `json:"pkg_name"`
	LatestRelease    string `json:"latest_release"`
	InstalledVersion string `json:"installed_version"`
}

func (r *Roster) Update() (*Updates, error) {
	ret := &Updates{}

	syncStat, err := r.SyncCheck()
	if err != nil {
		return nil, err
	}

	for _, stat := range syncStat {
		if stat.NeedSync {
			if err := r.Sync(RosterName(stat.RosterName), ROSTER_REPOS[RosterName(stat.RosterName)]); err != nil {
				return nil, err
			}
		}

		r.WalkPackageMeta(func(name string) bool {
			cache, err := r.LoadPackageCache(name)
			if err != nil {
				// failed to update cache
				return true
			}
			instVer, err := r.InstalledVersion(name)
			if err != nil {
				// not installed or error
				instVer = nil
			}
			latestVersion := cache.LatestVersion
			if instVer != nil {
				// installed
				if latestVersion != instVer.Version {
					ret.Upgradable = append(ret.Upgradable, &Upgradable{
						PkgName:          name,
						LatestRelease:    latestVersion,
						InstalledVersion: instVer.Version,
					})
				}
			}
			return true
		})
	}
	return ret, nil
}

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

func (r *Roster) LoadPackageMeta(pkgName string) (*PackageMeta, error) {
	return r.LoadPackageMetaRoster(ROSTER_CENTRAL, pkgName)
}

// LoadPackageMetaRoster loads package.yml file from the given package name.
// if the package.yml file is not found, it will return nil, and nil error.
// if the package.yml file is found, it will return the package meta info and nil error
// if the package.yml has an error, it will return the error.
func (r *Roster) LoadPackageMetaRoster(rosterName RosterName, pkgName string) (*PackageMeta, error) {
	path := filepath.Join(r.metaDir, string(rosterName), "projects", pkgName, "package.yml")
	if stat, err := os.Stat(path); err != nil || stat.IsDir() {
		path = filepath.Join(r.metaDir, string(rosterName), "projects", pkgName, "package.yaml")
		if stat, err := os.Stat(path); err != nil || stat.IsDir() {
			return nil, nil
		}
	}
	return LoadPackageMetaFile(path)
}

// Install installs the package to the distDir
// returns the installed symlink path '~/dist/<name>/current'
func (r *Roster) install0(name string, output io.Writer, env []string) error {
	meta, err := r.LoadPackageMeta(name)
	if err != nil {
		return err
	}
	cache, err := r.UpdatePackageCache(meta)
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

	switch dist.ArchiveExt {
	case ".zip":
		cmd := exec.Command("unzip", "-o", "-d", unarchiveDir, archiveFile)
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
			return err
		}
	case ".tar.gz":
		cmd := exec.Command("tar", "xf", archiveFile, "-C", unarchiveDir, "--strip-components", fmt.Sprintf("%d", dist.StripComponents))
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
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
		if sc, err := makeScriptFile(meta.InstallRecipe.Script, unarchiveDir, "__install__.sh"); err != nil {
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
	// remove archive file
	os.Remove(archiveFile)
	return nil
}

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
		if sc, err := makeScriptFile(meta.UninstallRecipe.Script, inst.Path, "__uninstall__.sh"); err != nil {
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

	if !filepath.IsAbs(inst.Path) || !strings.HasPrefix(inst.Path, r.distDir) {
		return fmt.Errorf("invalid installed path: %q", inst.Path)
	}
	if err := os.RemoveAll(inst.Path); err != nil {
		return err
	}
	os.RemoveAll(filepath.Dir(inst.Path))
	return err
}

func (r *Roster) WritePackageDistributionAvailability(pda *PackageDistributionAvailability) error {
	path := filepath.Join(r.metaDir, string(pda.rosterName), ".cache", pda.Name, fmt.Sprintf("%s.yml", pda.Version))
	return WritePackageDistributionAvailability(path, pda)
}
