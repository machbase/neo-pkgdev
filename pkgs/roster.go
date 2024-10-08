package pkgs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type RosterName string

const ROSTER_CENTRAL RosterName = "central"

var ROSTER_REPOS = map[RosterName]string{
	ROSTER_CENTRAL: "https://github.com/machbase/neo-pkg.git",
}

type Roster struct {
	metaDir             string
	distDir             string
	log                 Logger
	syncWhenInitialized bool
	experimental        bool
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
	if ret.log == nil {
		ret.log = NewLogger(LOG_NONE)
	}
	initialized := false
	for _, dir := range []string{metaDir, distDir} {
		if _, err := os.Stat(dir); err != nil {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
			initialized = true
		}
	}
	if initialized && ret.syncWhenInitialized {
		if err := ret.Sync(ROSTER_CENTRAL, ROSTER_REPOS[ROSTER_CENTRAL]); err != nil {
			// keep going, we can not stop if the sync fails by some reason.
			ret.log.Errorf("Sync error: %s roster %s", ROSTER_CENTRAL, err)
		}
	}
	return ret, nil
}

func WithLogger(logger Logger) RosterOption {
	return func(r *Roster) {
		r.log = logger
	}
}

func WithSyncWhenInitialized(flag bool) RosterOption {
	return func(r *Roster) {
		r.syncWhenInitialized = flag
	}
}

func WithExperimental(flag bool) RosterOption {
	return func(r *Roster) {
		r.experimental = flag
	}
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

func (r *Roster) WritePackageDistributionAvailability(pda []*PackageDistributionAvailability) error {
	if len(pda) == 0 {
		return nil
	}
	rosterName := pda[0].rosterName
	pkgName := pda[0].Name
	pkgVersion := pda[0].Version
	path := filepath.Join(r.metaDir, string(rosterName), ".cache", pkgName, fmt.Sprintf("%s.yml", pkgVersion))
	return WritePackageDistributionAvailability(path, pda)
}

func (r *Roster) LoadPackageDistributionAvailability(pkgName, pkgVersion string) ([]*PackageDistributionAvailability, error) {
	rosterName := string(ROSTER_CENTRAL)
	if strings.Contains(pkgName, "/") {
		toks := strings.SplitN(pkgName, "/", 2)
		rosterName = toks[0]
		pkgName = toks[1]
	}
	path := filepath.Join(r.metaDir, rosterName, ".cache", pkgName, fmt.Sprintf("%s.yml", pkgVersion))
	return ReadPackageDistributionAvailability(path)
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
