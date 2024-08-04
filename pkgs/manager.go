package pkgs

import (
	"io"

	"github.com/machbase/neo-server/mods/logging"
)

type PkgManager struct {
	log    logging.Log
	roster *Roster
}

func NewPkgManager(pkgsDir string) (*PkgManager, error) {
	roster, err := NewRoster(pkgsDir)
	if err != nil {
		return nil, err
	}
	return &PkgManager{
		log:    logging.GetLog("pkgmgr"),
		roster: roster,
	}, nil
}

func (pm *PkgManager) Sync() error {
	return pm.roster.Sync()
}

// if name is empty, it will return all featured packages
func (pm *PkgManager) Search(name string, possible int) (*PackageSearchResult, error) {
	if name == "" {
		prj, err := pm.roster.RootMeta()
		if err != nil {
			return nil, err
		}
		ret := &PackageSearchResult{}
		for _, pkg := range prj.Featured {
			if meta, err := pm.roster.LoadPackageMeta(pkg); err != nil {
				pm.log.Error("failed to load package meta", pkg, err)
			} else {
				cache, err := pm.roster.LoadPackageCache(pkg, meta, false)
				if err != nil {
					pm.log.Error("failed to load package cache", pkg, err)
				} else {
					ret.Possibles = append(ret.Possibles, cache)
				}
			}
			if possible > 0 && len(ret.Possibles) >= possible {
				break
			}
		}
		return ret, nil
	} else {
		return pm.roster.SearchPackage(name, possible)
	}
}

func (pm *PkgManager) Install(name string, output io.Writer) (*PackageCache, error) {
	err := pm.roster.Install(name, output)
	if err != nil {
		return nil, err
	}
	cache, err := pm.roster.cacheManagers[ROSTER_CENTRAL].ReadCache(name)
	if err != nil {
		return nil, err
	}

	pm.log.Info("installed", name, cache.InstalledVersion, cache.InstalledPath)
	return cache, nil
}

func (pm *PkgManager) Uninstall(name string, output io.Writer) (*PackageCache, error) {
	err := pm.roster.Uninstall(name, output)
	if err != nil {
		return nil, err
	}
	cache, err := pm.roster.cacheManagers[ROSTER_CENTRAL].ReadCache(name)
	if err != nil {
		return nil, err
	}
	pm.log.Info("uninstalled", name)
	return cache, nil
}

func (pm *PkgManager) Build(name string, version string) error {
	meta, err := pm.roster.LoadPackageMeta(name)
	if err != nil {
		return nil
	}
	builder, err := NewBuilder(
		meta, version,
		WithWorkDir(pm.roster.buildDir),
		WithDistDir(pm.roster.distDir),
	)
	if err != nil {
		return err
	}

	err = builder.Build(version)
	if err != nil {
		return err
	}
	return nil
}
