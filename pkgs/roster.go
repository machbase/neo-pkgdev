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
	"text/template"
	"time"

	"github.com/Masterminds/semver/v3"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"gopkg.in/yaml.v3"
)

type RosterName string

const ROSTER_CENTRAL RosterName = "central"

var ROSTER_REPOS = map[RosterName]string{
	ROSTER_CENTRAL: "https://github.com/machbase/neo-pkg.git",
}

type Roster struct {
	metaDir       string
	distDir       string
	buildDir      string
	cacheManagers map[RosterName]*CacheManager
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
	ret.buildDir = filepath.Join(metaDir, ".build")
	centralCacheDir := filepath.Join(metaDir, ".cache", string(ROSTER_CENTRAL))
	if err := os.MkdirAll(centralCacheDir, 0755); err != nil {
		return nil, err
	}
	ret.cacheManagers = map[RosterName]*CacheManager{
		ROSTER_CENTRAL: NewCacheManager(centralCacheDir),
	}
	return ret, nil
}

func (r *Roster) MetaDir(metaType RosterName) string {
	return filepath.Join(r.metaDir, string(metaType))
}

// WalkPackages walks all packages in the central repository.
// if callback returns false, it will stop walking.
func (r *Roster) WalkPackages(cb func(name string) bool) error {
	entries, err := os.ReadDir(filepath.Join(r.MetaDir(ROSTER_CENTRAL), "projects"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if !cb(entry.Name()) {
				return nil
			}
		}
	}
	return nil
}

type RootMeta struct {
	Featured []string
}

func (r *Roster) RootMeta() (*RootMeta, error) {
	path := filepath.Join(r.MetaDir(ROSTER_CENTRAL), "projects.yml")

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &RootMeta{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *Roster) SyncCheck() ([]*SyncCheck, error) {
	ret := []*SyncCheck{}
	for rosterName, rosterRepoUrl := range ROSTER_REPOS {
		if sc, err := r.SyncRosterCheck(rosterName, rosterRepoUrl); err != nil {
			return nil, err
		} else {
			ret = append(ret, sc)
		}
	}
	return ret, nil
}

type SyncCheck struct {
	RosterName   string
	NeedSync     bool
	LocalCommit  plumbing.Hash
	RemoteCommit plumbing.Hash
}

func (r *Roster) SyncRosterCheck(rosterName RosterName, rosterRepoUrl string) (*SyncCheck, error) {
	repo, err := git.PlainOpen(r.MetaDir(rosterName))
	if err != nil {
		return nil, err
	}
	headRef, err := repo.Head()
	if err != nil {
		return nil, err
	}
	remote := git.NewRemote(repo.Storer, &config.RemoteConfig{
		Name: string(git.DefaultRemoteName),
		URLs: []string{rosterRepoUrl},
	})
	remoteRefs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return nil, err
	}

	var remoteRef *plumbing.Reference
	for _, ref := range remoteRefs {
		refName := ref.Name()
		if !refName.IsBranch() {
			continue
		}
		if refName.Short() == "main" {
			remoteRef = ref
		}
	}
	ret := &SyncCheck{
		RosterName:   string(rosterName),
		LocalCommit:  headRef.Hash(),
		RemoteCommit: remoteRef.Hash(),
	}
	if headRef.Hash() != remoteRef.Hash() {
		ret.NeedSync = true
	}
	return ret, nil
}

func (r *Roster) Sync() error {
	for rosterName, rosterRepoUrl := range ROSTER_REPOS {
		if err := r.SyncRoster(rosterName, rosterRepoUrl); err != nil {
			return err
		}
	}
	return nil
}

func (r *Roster) SyncRoster(rosterName RosterName, rosterRepoUrl string) error {
	var repo *git.Repository
	var isBare = false
	if _, err := os.Stat(r.MetaDir(rosterName)); err != nil {
		repo, err = git.PlainClone(r.MetaDir(rosterName), isBare, &git.CloneOptions{
			URL:           rosterRepoUrl,
			RemoteName:    string(git.DefaultRemoteName),
			ReferenceName: plumbing.ReferenceName("refs/heads/main"),
			SingleBranch:  true,
			Depth:         1,
		})
		if err != nil {
			return err
		}
	} else {
		repo, err = git.PlainOpen(r.MetaDir(rosterName))
		if err != nil {
			return err
		}
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree error: %w", err)
	}
	err = w.Reset(&git.ResetOptions{Mode: git.HardReset})
	if err != nil {
		return fmt.Errorf("reset error: %w", err)
	}

	err = w.Pull(&git.PullOptions{
		RemoteURL:     rosterRepoUrl,
		RemoteName:    string(git.DefaultRemoteName),
		ReferenceName: plumbing.ReferenceName("refs/heads/main"),
		Depth:         0,
		Force:         true,
		SingleBranch:  true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("pull error: %w", err)
	}
	return nil
}

type Updates struct {
	Updated    []*Updated    `json:"updated"`
	Upgradable []*Upgradable `json:"upgradable"`
}

type Updated struct {
	RosterName    string `json:"-"`
	PkgName       string `json:"pkg_name"`
	LatestRelease string `json:"latest_release"`
}

type Upgradable struct {
	RosterName       string `json:"-"`
	PkgName          string `json:"pkg_name"`
	LatestRelease    string `json:"latest_release"`
	InstalledVersion string `json:"installed_version"`
}

func (r *Roster) Update() (*Updates, error) {
	ret := &Updates{}
	for rosterName, rosterRepoUrl := range ROSTER_REPOS {
		chk, err := r.SyncRosterCheck(rosterName, rosterRepoUrl)
		if err != nil {
			return nil, err
		}
		if chk.NeedSync {
			if err := r.SyncRoster(rosterName, rosterRepoUrl); err != nil {
				return nil, err
			}
		}
		r.WalkPackages(func(name string) bool {
			meta, err := r.LoadPackageMeta(name)
			if err != nil {
				return true
			}
			oldCache, _ := r.LoadPackageCache(name, meta, false)
			newCache, err := r.LoadPackageCache(name, meta, true)
			if err == nil {
				if oldCache == nil || oldCache.LatestReleaseTag != newCache.LatestReleaseTag {
					ret.Updated = append(ret.Updated, &Updated{
						RosterName:    string(rosterName),
						PkgName:       name,
						LatestRelease: newCache.LatestReleaseTag,
					})
				}
				if oldCache != nil && newCache != nil && oldCache.InstalledVersion != "" && newCache.LatestRelease != oldCache.InstalledVersion {
					ret.Upgradable = append(ret.Upgradable, &Upgradable{
						RosterName:       string(rosterName),
						PkgName:          name,
						LatestRelease:    newCache.LatestReleaseTag,
						InstalledVersion: oldCache.InstalledVersion,
					})
				}
			}
			return true
		})
	}
	return ret, nil
}

type Installed struct {
	PkgName string        `json:"pkg_name"`
	Success bool          `json:"success"`
	Err     error         `json:"error,omitempty"`
	Cache   *PackageCache `json:"installed,omitempty"`
	Output  string        `json:"output,omitempty"`
}

func (r *Roster) Upgrade(pkgs []string) []*Installed {
	ret := make([]*Installed, len(pkgs))
	for i, name := range pkgs {
		output := &strings.Builder{}
		if err := r.Install(name, output); err != nil {
			ret[i] = &Installed{
				PkgName: name,
				Success: false,
				Err:     err,
				Output:  output.String(),
			}
		} else {
			meta, _ := r.LoadPackageMeta(name)
			cache, _ := r.LoadPackageCache(name, meta, false)
			ret[i] = &Installed{
				PkgName: name,
				Success: true,
				Cache:   cache,
				Output:  output.String(),
			}
		}
	}
	return ret
}

func (r *Roster) InstallDir(name string) string {
	return filepath.Join(r.distDir, name, "current")
}

func (r *Roster) ReadCache(name string) (*PackageCache, error) {
	return r.cacheManagers[ROSTER_CENTRAL].ReadCache(name)
}

func (r *Roster) LoadPackageMeta(pkgName string) (*PackageMeta, error) {
	return r.LoadPackageMetaRoster(ROSTER_CENTRAL, pkgName)
}

// LoadPackageMetaRoster loads package.yml file from the given package name.
// if the package.yml file is not found, it will return nil, and nil error.
// if the package.yml file is found, it will return the package meta info and nil error
// if the package.yml has an error, it will return the error.
func (r *Roster) LoadPackageMetaRoster(rosterName RosterName, pkgName string) (*PackageMeta, error) {
	path := filepath.Join(r.MetaDir(rosterName), "projects", pkgName, "package.yml")
	if stat, err := os.Stat(path); err != nil || stat.IsDir() {
		path = filepath.Join(r.MetaDir(rosterName), "projects", pkgName, "package.yaml")
		if stat, err := os.Stat(path); err != nil || stat.IsDir() {
			return nil, nil
		}
	}
	ret, err := parsePackageMetaFile(path)
	if ret != nil {
		ret.rosterName = rosterName
	}
	return ret, err
}

func (r *Roster) LoadPackageCache(name string, meta *PackageMeta, forceRefresh bool) (*PackageCache, error) {
	// if this is the first time to load the package cache,
	// it will receive the error of "file not found".
	cache, _ := r.cacheManagers[meta.rosterName].ReadCache(name)
	if !forceRefresh {
		return cache, nil
	}

	if cache == nil {
		cache = &PackageCache{
			Name: name,
		}
	}
	org, repo, err := GithubSplitPath(meta.Distributable.Github)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(10) * time.Second,
	}

	var ghRepo *GhRepoInfo
	if lr, err := GithubRepoInfo(httpClient, org, repo); err != nil {
		return cache, err
	} else {
		ghRepo = lr
	}

	var ghRelease *GhReleaseInfo
	if lr, err := GithubLatestReleaseInfo(httpClient, org, repo); err != nil {
		return cache, err
	} else {
		ghRelease = lr
	}

	tmpl, err := template.New("url").Parse(meta.Distributable.Url)
	if err != nil {
		return cache, err
	}
	buff := &strings.Builder{}
	tmpl.Execute(buff, map[string]string{
		"tag":     ghRelease.TagName,
		"version": strings.TrimPrefix(ghRelease.TagName, "v"),
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	})

	if _, err := semver.NewVersion(ghRelease.Name); err != nil {
		return nil, err
	}

	cache.Name = name
	cache.Github = ghRepo
	cache.LatestRelease = ghRelease.Name
	cache.LatestReleaseTag = ghRelease.TagName
	cache.StripComponents = meta.Distributable.StripComponents
	cache.PublishedAt = ghRelease.PublishedAt
	cache.Url = buff.String()
	cache.CachedAt = time.Now()

	thisPkgDir := filepath.Join(r.distDir, name)
	currentVerDir := filepath.Join(thisPkgDir, "current")
	if _, err := os.Stat(currentVerDir); err == nil {
		cache.InstalledPath = currentVerDir
	}
	current, err := os.Readlink(currentVerDir)
	if err == nil {
		linkName := filepath.Base(current)
		if _, err := semver.NewVersion(linkName); err == nil {
			cache.InstalledVersion = linkName
		}
	}
	return cache, r.cacheManagers[ROSTER_CENTRAL].WriteCache(cache)
}

// Install installs the package to the distDir
// returns the installed symlink path '~/dist/<name>/current'
func (r *Roster) Install(name string, output io.Writer) error {
	meta, err := r.LoadPackageMeta(name)
	if err != nil {
		return err
	}
	cache, err := r.LoadPackageCache(name, meta, true)
	if err != nil {
		return err
	}

	force := true
	fileBase := ""
	fileExt := ""
	releaseFilename := strings.TrimPrefix(cache.LatestRelease, "v")
	if cache.Url != "" {
		// from direct url
		fileBase = filepath.Base(cache.Url)
		fileExt = filepath.Ext(fileBase)
	} else {
		// from s3
		fileBase = fmt.Sprintf("%s-%s.tar.gz", cache.Github.Repo, releaseFilename)
		fileExt = ".tar.gz"
	}
	thisPkgDir := filepath.Join(r.distDir, cache.Name)
	archiveFile := filepath.Join(thisPkgDir, fmt.Sprintf("%s%s", releaseFilename, fileExt))
	unarchiveDir := filepath.Join(thisPkgDir, releaseFilename)
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
		bucket := "p-edge-packages"
		region := "ap-northeast-2"
		s3url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/neo-pkg/%s/%s/%s",
			bucket, region,
			cache.Github.Organization, cache.Github.Repo, fileBase)
		if u, err := url.Parse(s3url); err != nil {
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

	nBytes, err := io.Copy(download, rsp.Body)
	if err != nil {
		return err
	}
	download.Close()
	fmt.Println("Downloaded", nBytes, "bytes")

	switch fileExt {
	case ".zip":
		cmd := exec.Command("unzip", "-o", "-d", unarchiveDir, archiveFile)
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
			return err
		}
	case ".tar.gz":
		cmd := exec.Command("tar", "xf", archiveFile, "-C", unarchiveDir, "--strip-components", fmt.Sprintf("%d", cache.StripComponents))
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(currentVerDir); err == nil {
		if err := os.Remove(currentVerDir); err != nil {
			return err
		}
	}
	err = os.Symlink(releaseFilename, currentVerDir)
	if err == nil {
		cache.InstalledVersion = cache.LatestRelease
		cache.InstalledPath = currentVerDir
		cache.CachedAt = time.Now()
		err = r.cacheManagers[meta.rosterName].WriteCache(cache)
	}
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
			err = cmd.Run()
			if err != nil {
				return err
			}
		}
		os.Remove(filepath.Join(unarchiveDir, "__install__.sh"))
	}
	return nil
}

func (r *Roster) Uninstall(name string, output io.Writer) error {
	meta, err := r.LoadPackageMeta(name)
	if err != nil {
		return err
	}
	cache, err := r.LoadPackageCache(name, meta, true)
	if err != nil {
		return err
	}

	if meta.UninstallRecipe != nil {
		if sc, err := makeScriptFile(meta.UninstallRecipe.Script, cache.InstalledPath, "__uninstall__.sh"); err != nil {
			return err
		} else {
			cmd := exec.Command("sh", "-c", sc)
			cmd.Dir = cache.InstalledPath
			cmd.Stdout = output
			cmd.Stderr = output
			err = cmd.Run()
			if err != nil {
				return err
			}
			os.Remove(filepath.Join(cache.InstalledPath, "__uninstall__.sh"))
		}
	}

	if !filepath.IsAbs(cache.InstalledPath) || !strings.HasPrefix(cache.InstalledPath, r.distDir) {
		return fmt.Errorf("invalid installed path: %q", cache.InstalledPath)
	}
	if err := os.RemoveAll(cache.InstalledPath); err != nil {
		return err
	}
	os.RemoveAll(filepath.Dir(cache.InstalledPath))
	cache.InstalledPath = ""
	cache.InstalledVersion = ""
	err = r.cacheManagers[meta.rosterName].WriteCache(cache)
	return err
}
