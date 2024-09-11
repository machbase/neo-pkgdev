package pkgs

import (
	"runtime"
	"slices"
	"strings"
)

type PackageSearch struct {
	Name  string
	Score float32
	Cache *PackageCache
}

type PackageSearchResult struct {
	ExactMatch *PackageCache   `json:"exact"`
	Possibles  []*PackageCache `json:"possibles"`
	Installed  []*PackageCache `json:"installed,omitempty"`
	Broken     []string        `json:"broken,omitempty"`
}

// if name is empty, it will return all featured packages
func (r *Roster) Search(name string, possible int) (*PackageSearchResult, error) {
	ret := &PackageSearchResult{}
	if name == "" {
		inst, err := r.InstalledPackages()
		if err != nil {
			// if update is needed
			_, err := r.Update()
			if err != nil {
				return nil, err
			}
			inst, err = r.InstalledPackages()
			if err != nil {
				return nil, err
			}
		}
		for _, pkg := range inst.Installed {
			cache, err := r.LoadPackageCache(pkg)
			if err != nil {
				ret.Broken = append(ret.Broken, pkg)
			} else {
				if !cache.Support(runtime.GOOS, runtime.GOARCH) {
					continue
				}
				ret.Installed = append(ret.Installed, cache)
			}
		}

		prj, err := r.FeaturedPackages()
		if err != nil {
			return nil, err
		}
		for _, pkg := range prj.Featured {
			if len(inst.Installed) > 0 && slices.Contains(inst.Installed, pkg) {
				continue
			}
			cache, err := r.LoadPackageCache(pkg)
			if err != nil {
				ret.Broken = append(ret.Broken, pkg)
			} else {
				if !cache.Support(runtime.GOOS, runtime.GOARCH) {
					continue
				}
				ret.Possibles = append(ret.Possibles, cache)
			}
			if possible > 0 && len(ret.Possibles) >= possible {
				break
			}
		}
	} else {
		if result, err := r.SearchPackage(name, possible); err != nil {
			return nil, err
		} else {
			ret = result
		}
	}
	pkgs := []*PackageCache{}
	if ret.ExactMatch != nil {
		pkgs = append(pkgs, ret.ExactMatch)
	}
	pkgs = append(pkgs, ret.Possibles...)
	pkgs = append(pkgs, ret.Installed...)
	// check if installed
	for _, p := range pkgs {
		r.CheckInstalledPackage(p)
	}
	// check if installable
	for _, p := range pkgs {
		r.CheckAvailabilityPackage(p)
	}
	return ret, nil
}

func (r *Roster) CheckAvailabilityPackage(cache *PackageCache) error {
	avails, err := r.LoadPackageDistributionAvailability(cache.Name, cache.LatestVersion)
	if err != nil {
		return err
	}
	for _, a := range avails {
		if a.PlatformOS == "" && a.PlatformArch == "" {
			cache.LatestReleaseSize = a.ContentLength
			break
		} else if a.PlatformOS == runtime.GOOS && a.PlatformArch == runtime.GOARCH {
			cache.LatestReleaseSize = a.ContentLength
			break
		}
	}
	return nil
}

func (r *Roster) CheckInstalledPackage(cache *PackageCache) error {
	inst, err := r.InstalledVersion(cache.Name)
	if err != nil {
		return err
	}
	if inst != nil {
		cache.InstalledVersion = inst.Version
		cache.InstalledPath = inst.Path
		cache.InstalledBackend = inst.HasBackend
		cache.InstalledFrontend = inst.HasFrontend
		cache.WorkInProgress = inst.WorkInProgress
	}
	return nil
}

// Search package info by name, if it finds the package, return the package info.
// if not found it will return similar package names.
// if there is no similar package names, it will return empty string slice.
// if possibles is 0, it will only return exact match.
func (r *Roster) SearchPackage(name string, possibles int) (*PackageSearchResult, error) {
	nfo, err := r.LoadPackageMeta(name)
	if err != nil {
		return nil, err
	}
	ret := &PackageSearchResult{}
	if nfo != nil {
		cache, err := r.LoadPackageCache(name)
		if err != nil {
			return nil, err
		}
		if cache.Support(runtime.GOOS, runtime.GOARCH) {
			ret.ExactMatch = cache
		}
	}
	if possibles == 0 {
		return ret, nil
	}
	// search similar package names
	candidates := []*PackageSearch{}
	r.WalkPackageCache(func(nm string) bool {
		if !r.experimental {
			// skip alpha version on non-experimental mode
			cache, err := r.LoadPackageCache(nm)
			if err != nil {
				return true
			}
			if strings.Contains(cache.LatestVersion, "alpha") {
				return true
			}
		}
		if ret.ExactMatch != nil && ret.ExactMatch.Name == nm {
			return true
		}
		score := CompareTwoStrings(strings.ToLower(nm), name)
		if score > 0.1 {
			cache, err := r.LoadPackageCache(nm)
			if err != nil || !cache.Support(runtime.GOOS, runtime.GOARCH) {
				return true
			}
			candidates = append(candidates, &PackageSearch{Name: nm, Score: score, Cache: cache})
		}
		return true
	})

	slices.SortFunc(candidates, func(a, b *PackageSearch) int {
		if a.Score > b.Score {
			return -1
		} else if a.Score < b.Score {
			return 1
		}
		return 0
	})
	if len(candidates) > possibles {
		candidates = candidates[:possibles]
	}
	for _, c := range candidates {
		ret.Possibles = append(ret.Possibles, c.Cache)
	}
	return ret, nil
}
