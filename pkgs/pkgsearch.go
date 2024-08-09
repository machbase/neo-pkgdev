package pkgs

import (
	"slices"
	"strings"
)

type PackageSearch struct {
	Name  string
	Score float32
}

type PackageSearchResult struct {
	ExactMatch *PackageCache   `json:"exact"`
	Possibles  []*PackageCache `json:"possibles"`
	Broken     []string        `json:"broken"`
}

// if name is empty, it will return all featured packages
func (r *Roster) Search(name string, possible int) (*PackageSearchResult, error) {
	if name == "" {
		prj, err := r.FeaturedPackages()
		if err != nil {
			return nil, err
		}
		ret := &PackageSearchResult{}
		for _, pkg := range prj.Featured {
			cache, err := r.LoadPackageCache(pkg)
			if err != nil {
				ret.Broken = append(ret.Broken, pkg)
			} else {
				ret.Possibles = append(ret.Possibles, cache)
			}
			if possible > 0 && len(ret.Possibles) >= possible {
				break
			}
		}
		return ret, nil
	} else {
		return r.SearchPackage(name, possible)
	}
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
		ret.ExactMatch = cache
	}
	if possibles == 0 {
		return ret, nil
	}
	// search similar package names
	candidates := []*PackageSearch{}
	r.WalkPackageCache(func(nm string) bool {
		score := CompareTwoStrings(strings.ToLower(nm), name)
		if score > 0.1 {
			candidates = append(candidates, &PackageSearch{Name: nm, Score: score})
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
		cache, err := r.LoadPackageCache(c.Name)
		if err != nil {
			continue
		}
		ret.Possibles = append(ret.Possibles, cache)
	}
	return ret, nil
}
