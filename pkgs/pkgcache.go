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
	Platforms         []string    `yaml:"platforms" json:"platforms"`
	rosterName        RosterName  `yaml:"-" json:"-"`
	// this field is not saved in cache file, but includes in json api response
	InstalledVersion  string `yaml:"-" json:"installed_version"`
	InstalledPath     string `yaml:"-" json:"installed_path"`
	InstalledFrontend bool   `yaml:"-" json:"installed_frontend"`
	InstalledBackend  bool   `yaml:"-" json:"installed_backend"`
	WorkInProgress    bool   `yaml:"-" json:"work_in_progress"`
}

func (cache *PackageCache) Support(platformOS string, platformArch string) bool {
	if len(cache.Platforms) == 0 {
		return true
	}
	demand := fmt.Sprintf("%s/%s", platformOS, platformArch)
	for _, platform := range cache.Platforms {
		if platform == "/" {
			return true
		}
		if platform == demand {
			return true
		}
	}
	return false
}
func (cache *PackageCache) RemoteDistribution() ([]*PackageDistribution, error) {
	ret := []*PackageDistribution{}
	platforms := []string{}
	if len(cache.Platforms) == 0 {
		// no platforms specified, use default
		platforms = append(platforms, "/")
	} else {
		platforms = cache.Platforms
	}
	for _, platform := range platforms {
		platformOS := ""
		platformArch := ""
		osArch := strings.SplitN(platform, "/", 2)
		if len(osArch) != 2 {
			return nil, fmt.Errorf("invalid platform: %s", platform)
		} else {
			platformOS = osArch[0]
			platformArch = osArch[1]
		}

		pd := &PackageDistribution{Name: cache.Name, StripComponents: cache.StripComponents, rosterName: cache.rosterName}
		pd.PlatformOS = platformOS
		pd.PlatformArch = platformArch
		if cache.Url != "" {
			// from direct url
			pd.Url = cache.Url
			pd.ArchiveBase = filepath.Base(cache.Url)
			pd.ArchiveExt = filepath.Ext(pd.ArchiveBase)
			pd.UnarchiveDir = strings.TrimSuffix(pd.ArchiveBase, pd.ArchiveExt)
			pd.ArchiveSize = cache.LatestReleaseSize
		} else {
			// from s3
			releaseFilename := cache.LatestVersion
			if platformOS != "" && platformArch != "" {
				pd.ArchiveBase = fmt.Sprintf("%s-%s-%s-%s.tar.gz", cache.Github.Repo, releaseFilename, platformOS, platformArch)
			} else {
				pd.ArchiveBase = fmt.Sprintf("%s-%s.tar.gz", cache.Github.Repo, releaseFilename)
			}
			pd.ArchiveExt = ".tar.gz"
			pd.UnarchiveDir = releaseFilename
			pd.ArchiveSize = cache.LatestReleaseSize

			bucket := "p-edge-packages"
			region := "ap-northeast-2"
			pd.Url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/neo-pkg/%s/%s/%s",
				bucket, region,
				cache.Github.Organization, cache.Github.Repo, pd.ArchiveBase)

		}
		ret = append(ret, pd)
	}
	return ret, nil
}

type InstalledVersion struct {
	Name           string `yaml:"name" json:"name"`
	Version        string `yaml:"version" json:"version"`
	Path           string `yaml:"path" json:"path"`
	CurrentPath    string `yaml:"current" json:"current"`
	HasBackend     bool   `yaml:"has_backend" json:"has_backend"`
	HasFrontend    bool   `yaml:"has_frontend" json:"has_frontend"`
	WorkInProgress bool   `yaml:"work_in_progress" json:"work_in_progress"`
}

func (roster *Roster) InstalledVersion(pkgName string) (*InstalledVersion, error) {
	rosterName, name := RosterNames(pkgName)
	var thisPkgDir string
	if rosterName == ROSTER_CENTRAL {
		thisPkgDir = filepath.Join(roster.distDir, name)
	} else {
		thisPkgDir = filepath.Join(roster.distDir, string(rosterName), name)
	}
	wip := false
	if _, err := os.Stat(filepath.Join(thisPkgDir, "wip")); err == nil {
		wip = true
	}
	currentVerDir := filepath.Join(thisPkgDir, "current")
	if _, err := os.Stat(currentVerDir); err == nil {
		ret := &InstalledVersion{
			Name:           pkgName,
			WorkInProgress: wip,
		}
		ret.CurrentPath = currentVerDir
		current, err := Readlink(currentVerDir)
		if err != nil {
			return nil, fmt.Errorf("package current link not found")
		}
		linkName := filepath.Base(current)
		ret.Version = linkName
		ret.Path = filepath.Join(thisPkgDir, linkName)
		if _, err := os.Stat(filepath.Join(ret.Path, ".backend.yml")); err == nil {
			ret.HasBackend = true
		}
		if _, err := os.Stat(filepath.Join(ret.Path, "index.html")); err == nil {
			ret.HasFrontend = true
		}
		return ret, nil
	} else {
		return nil, fmt.Errorf("package %q not installed", pkgName)
	}
}

func (roster *Roster) WritePackageCache(cache *PackageCache) error {
	cachePath := filepath.Join(roster.metaDir, string(cache.rosterName), ".cache", cache.Name, "cache.yml")
	return WritePackageCacheFile(cachePath, cache)
}

// Refresh the package cache.
// It is caller's responsibility to write the new cache to the file.
func (roster *Roster) UpdatePackageCache(meta *PackageMeta) (*PackageCache, error) {
	// if this is the first time to load the package cache,
	// it will receive the error of "file not found".
	cache := &PackageCache{
		Name:       meta.pkgName,
		Platforms:  meta.Platforms,
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
	return cache, err
}

// func (roster *Roster) SavePackageDistributionAvailability(pda []*PackageDistributionAvailability) error {
// 	if len(pda) == 0 {
// 		return nil
// 	}
// 	rosterName := string(pda[0].rosterName)
// 	pkgName := pda[0].Name
// 	pkgVersion := pda[0].Version
// 	cacheDir := filepath.Join(roster.metaDir, rosterName, ".cache", pkgName)
// 	if _, err := os.Stat(cacheDir); err != nil {
// 		if os.IsNotExist(err) {
// 			if err := os.MkdirAll(cacheDir, 0755); err != nil {
// 				return err
// 			}
// 		} else {
// 			return err
// 		}
// 	}
// 	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s.yml", pkgVersion))
// 	return WritePackageDistributionAvailability(cacheFile, pda)
// }

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

func WritePackageDistributionAvailability(path string, pda []*PackageDistributionAvailability) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}
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
	PlatformOS      string     `json:"platform_os"`
	PlatformArch    string     `json:"platform_arch"`
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
		Name:         pd.Name,
		Version:      pd.UnarchiveDir,
		PlatformOS:   pd.PlatformOS,
		PlatformArch: pd.PlatformArch,
		DistUrl:      pd.Url,
		StatusCode:   rsp.StatusCode,
		rosterName:   pd.rosterName,
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
	PlatformOS    string     `yaml:"platform_os"`
	PlatformArch  string     `yaml:"platform_arch"`
	DistUrl       string     `yaml:"dist_url"`
	StatusCode    int        `yaml:"status_code"`
	Available     bool       `yaml:"available"`
	ContentLength int64      `yaml:"content_length"`
	rosterName    RosterName `yaml:"-"`
}

func (lchk *PackageDistributionAvailability) String() string {
	return fmt.Sprint(lchk.Name, " ", lchk.Version, " ", lchk.DistUrl, " ", lchk.StatusCode, lchk.ContentLength)
}
