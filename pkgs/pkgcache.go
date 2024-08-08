package pkgs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	Url               string      `yaml:"url" json:"url"`
	StripComponents   int         `yaml:"strip_components" json:"strip_components"`
	CachedAt          time.Time   `yaml:"cached_at" json:"cached_at"`
	InstalledVersion  string      `yaml:"installed_version" json:"installed_version"`
	InstalledPath     string      `yaml:"installed_path" json:"installed_path"`
}

type PackageDistribution struct {
	Url             string `json:"url"`
	ArchiveBase     string `json:"archive_base"`
	ArchiveExt      string `json:"archive_ext"`
	ArchiveSize     int64  `json:"archive_size"`
	UnarchiveDir    string `json:"unarchive_base"`
	StripComponents int    `json:"strip_components"`
}

func (cache *PackageCache) RemoteDistribution() (*PackageDistribution, error) {
	ret := &PackageDistribution{StripComponents: cache.StripComponents}
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

type CacheManager struct {
	cacheDir string
}

func NewCacheManager(cacheDir string) *CacheManager {
	return &CacheManager{cacheDir: cacheDir}
}

func (cm *CacheManager) ReadCache(name string) (*PackageCache, error) {
	path := filepath.Join(cm.cacheDir, name, "cache.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &PackageCache{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (cm *CacheManager) WriteCache(cache *PackageCache) error {
	cacheDir := filepath.Join(cm.cacheDir, cache.Name)
	if _, err := os.Stat(cacheDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(cacheDir, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	cacheFile := filepath.Join(cacheDir, "cache.yml")
	content, err := yaml.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, content, 0644)
}

func (cm *CacheManager) Walk(cb func(name string) bool) error {
	entries, err := os.ReadDir(cm.cacheDir)
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
