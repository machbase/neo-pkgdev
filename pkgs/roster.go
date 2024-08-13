package pkgs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func (r *Roster) WritePackageDistributionAvailability(pda *PackageDistributionAvailability) error {
	path := filepath.Join(r.metaDir, string(pda.rosterName), ".cache", pda.Name, fmt.Sprintf("%s.yml", pda.Version))
	return WritePackageDistributionAvailability(path, pda)
}

func MakeScriptFile(script []string, destDir string, filename string) (string, error) {
	var buildScript, _ = filepath.Abs(filepath.Join(destDir, filename))
	f, err := os.OpenFile(buildScript, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if runtime.GOOS != "windows" {
		fmt.Fprintln(f, "set -e")
	}
	for _, line := range script {
		fmt.Fprintln(f, line)
	}
	return buildScript, nil
}
