package pkgs

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"
)

type PackageCache struct {
	Name              string      `yaml:"name" json:"name"`
	Github            *GhRepoInfo `yaml:"github" json:"github"`
	LatestVersion     string      `yaml:"latest_version" json:"latest_version"`
	LatestRelease     string      `yaml:"latest_release" json:"latest_release"`
	LatestReleaseTag  string      `yaml:"latest_release_tag" json:"latest_release_tag"`
	LatestReleaseSize int64       `yaml:"latest_release_size" json:"latest_release_size"`
	PublishedAt       time.Time   `yaml:"published_at" json:"published_at"`
	Url               string      `yaml:"url,omitempty" json:"url,omitempty"`
	StripComponents   int         `yaml:"strip_components" json:"strip_components"`
	rosterName        RosterName  `yaml:"-" json:"-"`
	// this field is not saved in cache file, but includes in json api response
	InstalledVersion string `yaml:"-" json:"installed_version"`
	InstalledPath    string `yaml:"-" json:"installed_path"`
}

func (cache *PackageCache) RemoteDistribution() (*PackageDistribution, error) {
	ret := &PackageDistribution{Name: cache.Name, StripComponents: cache.StripComponents, rosterName: cache.rosterName}
	if cache.Url != "" {
		// from direct url
		ret.Url = cache.Url
		ret.ArchiveBase = filepath.Base(cache.Url)
		ret.ArchiveExt = filepath.Ext(ret.ArchiveBase)
		ret.UnarchiveDir = strings.TrimSuffix(ret.ArchiveBase, ret.ArchiveExt)
		ret.ArchiveSize = cache.LatestReleaseSize
	} else {
		// from s3
		releaseFilename := cache.LatestVersion
		ret.ArchiveBase = fmt.Sprintf("%s-%s.tar.gz", cache.Github.Repo, releaseFilename)
		ret.ArchiveExt = ".tar.gz"
		ret.UnarchiveDir = releaseFilename
		ret.ArchiveSize = cache.LatestReleaseSize

		bucket := "p-edge-packages"
		region := "ap-northeast-2"
		ret.Url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/neo-pkg/%s/%s/%s",
			bucket, region,
			cache.Github.Organization, cache.Github.Repo, ret.ArchiveBase)

	}
	return ret, nil
}

type InstalledVersion struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Path        string `yaml:"path" json:"path"`
	CurrentPath string `yaml:"current" json:"current"`
}

func (roster *Roster) InstalledVersion(pkgName string) (*InstalledVersion, error) {
	rosterName, name := RosterNames(pkgName)
	var thisPkgDir string
	if rosterName == ROSTER_CENTRAL {
		thisPkgDir = filepath.Join(roster.distDir, name)
	} else {
		thisPkgDir = filepath.Join(roster.distDir, string(rosterName), name)
	}
	currentVerDir := filepath.Join(thisPkgDir, "current")
	if _, err := os.Stat(currentVerDir); err == nil {
		ret := &InstalledVersion{
			Name: pkgName,
		}
		ret.CurrentPath = currentVerDir
		current, err := os.Readlink(currentVerDir)
		if err != nil {
			return nil, fmt.Errorf("package current link not found")
		}
		linkName := filepath.Base(current)
		ret.Version = linkName
		ret.Path = filepath.Join(thisPkgDir, linkName)
		return ret, nil
	} else {
		return nil, fmt.Errorf("package not installed")
	}
}

func (roster *Roster) UpdatePackageCache(meta *PackageMeta) (*PackageCache, error) {
	cachePath := filepath.Join(roster.metaDir, string(meta.rosterName), ".cache", meta.pkgName, "cache.yml")
	// if this is the first time to load the package cache,
	// it will receive the error of "file not found".
	cache := &PackageCache{
		Name:       meta.pkgName,
		rosterName: meta.rosterName,
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

	if _, err := semver.NewVersion(ghRelease.Name); err != nil {
		return nil, err
	}

	// version check
	if cache.LatestReleaseTag != ghRelease.TagName {
		cache.Github = ghRepo
		cache.LatestVersion = strings.TrimPrefix(strings.TrimPrefix(ghRelease.TagName, "v"), "V")
		cache.LatestRelease = ghRelease.Name
		cache.LatestReleaseTag = ghRelease.TagName
		cache.StripComponents = meta.Distributable.StripComponents
		cache.PublishedAt = ghRelease.PublishedAt
	}

	if meta.Distributable.Url != "" {
		tmpl, err := template.New("url").Parse(meta.Distributable.Url)
		if err != nil {
			return cache, err
		}
		buff := &strings.Builder{}
		version := strings.TrimPrefix(ghRelease.TagName, "v")
		version = strings.TrimPrefix(version, "V")
		tmpl.Execute(buff, map[string]string{
			"tag":     ghRelease.TagName,
			"version": version,
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		})
		cache.Url = buff.String()
	}

	err = WritePackageCacheFile(cachePath, cache)
	return cache, err
}

func (roster *Roster) SavePackageDistributionAvailability(pda *PackageDistributionAvailability) error {
	cacheDir := filepath.Join(roster.metaDir, string(pda.rosterName), ".cache", pda.Name)
	if _, err := os.Stat(cacheDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(cacheDir, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s.yml", pda.Version))
	return WritePackageDistributionAvailability(cacheFile, pda)
}

func (roster *Roster) LoadPackageCache(pkgName string) (*PackageCache, error) {
	var rosterName = ROSTER_CENTRAL
	rosterName, pkgName = RosterNames(pkgName)
	cachePath := filepath.Join(roster.metaDir, string(rosterName), ".cache", pkgName, "cache.yml")
	return ReadPackageCacheFile(cachePath)
}

func ReadPackageCacheFile(path string) (*PackageCache, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &PackageCache{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	rosterName := filepath.Base(filepath.Dir(filepath.Dir(path)))
	ret.rosterName = RosterName(rosterName)
	return ret, nil
}

func WritePackageCacheFile(path string, cache *PackageCache) error {
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	content, err := yaml.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func WritePackageDistributionAvailability(path string, pda *PackageDistributionAvailability) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(pda); err != nil {
		return err
	}
	return nil
}

type PackageDistribution struct {
	Name            string     `json:"name"`
	Url             string     `json:"url"`
	ArchiveBase     string     `json:"archive_base"`
	ArchiveExt      string     `json:"archive_ext"`
	ArchiveSize     int64      `json:"archive_size"`
	UnarchiveDir    string     `json:"unarchive_base"`
	StripComponents int        `json:"strip_components"`
	rosterName      RosterName `json:"-"`
}

func (pd *PackageDistribution) CheckAvailability(httpClient *http.Client) (*PackageDistributionAvailability, error) {
	rsp, err := httpClient.Head(pd.Url)
	if err != nil {
		return nil, err
	}
	rsp.Body.Close()
	ret := &PackageDistributionAvailability{
		Name:       pd.Name,
		Version:    pd.UnarchiveDir,
		DistUrl:    pd.Url,
		StatusCode: rsp.StatusCode,
		rosterName: pd.rosterName,
	}
	if rsp.StatusCode == 200 {
		if cl := rsp.Header.Get("Content-Length"); cl != "" {
			if v, ok := strconv.ParseInt(cl, 10, 64); ok == nil {
				ret.ContentLength = v
				ret.Available = true
			}
		}
	}
	return ret, nil
}

type PackageDistributionAvailability struct {
	Name          string     `yaml:"name"`
	Version       string     `yaml:"version"`
	DistUrl       string     `yaml:"dist_url"`
	StatusCode    int        `yaml:"status_code"`
	Available     bool       `yaml:"available"`
	ContentLength int64      `yaml:"content_length"`
	rosterName    RosterName `yaml:"-"`
}

func (lchk *PackageDistributionAvailability) String() string {
	return fmt.Sprint(lchk.Name, " ", lchk.Version, " ", lchk.DistUrl, " ", lchk.StatusCode, lchk.ContentLength)
}
