package pkgs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"gopkg.in/yaml.v3"
)

type PackageMeta struct {
	Distributable   Distributable    `yaml:"distributable" json:"distributable"`
	Description     string           `yaml:"description" json:"description"`
	Platforms       []string         `yaml:"platforms" json:"platforms"`
	BuildRecipe     BuildRecipe      `yaml:"build" json:"build"`
	Provides        []string         `yaml:"provides" json:"provides"`
	TestRecipe      *TestRecipe      `yaml:"test,omitempty" json:"test,omitempty"`
	InstallRecipe   *InstallRecipe   `yaml:"install,omitempty" json:"install,omitempty"`
	UninstallRecipe *UninstallRecipe `yaml:"uninstall,omitempty" json:"uninstall,omitempty"`

	rosterName RosterName `json:"-"`
	pkgName    string     `json:"-"`
}

type Distributable struct {
	Github          string `yaml:"github"`
	Url             string `yaml:"url"`
	StripComponents int    `yaml:"strip_components"`
}

type BuildRecipe struct {
	Script []string `yaml:"script"`
	Env    []string `yaml:"env"`
}

type TestRecipe struct {
	Script []string `yaml:"script"`
	Env    []string `yaml:"env"`
}

type InstallRecipe struct {
	Script []string `yaml:"script"`
	Env    []string `yaml:"env"`
}

type UninstallRecipe struct {
	Script []string `yaml:"script"`
	Env    []string `yaml:"env"`
}

func LoadPackageMetaFile(path string) (*PackageMeta, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &PackageMeta{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	ret.pkgName = filepath.Base(filepath.Dir(path))
	ret.rosterName = RosterName(filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path)))))
	return ret, nil
}

func RosterNames(pkgName string) (RosterName, string) {
	var rosterName = ROSTER_CENTRAL
	if strings.Contains(pkgName, "/") {
		splits := strings.Split(pkgName, "/")
		rosterName = RosterName(splits[0])
		pkgName = splits[1]
	}
	return rosterName, pkgName
}

type FeaturedPackages struct {
	Featured []string
}

func (r *Roster) FeaturedPackages() (*FeaturedPackages, error) {
	path := filepath.Join(r.metaDir, string(ROSTER_CENTRAL), "projects.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &FeaturedPackages{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// WalkPackages walks all caches.
// if callback returns false, it will stop walking.
func (roster *Roster) WalkPackageCache(cb func(pkgName string) bool) error {
	for _, rosterName := range []RosterName{ROSTER_CENTRAL} {
		cacheDir := filepath.Join(roster.metaDir, string(rosterName), ".cache")
		entries, err := os.ReadDir(cacheDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				pkgName := entry.Name()
				if rosterName != ROSTER_CENTRAL {
					pkgName = fmt.Sprintf("%s/%s", rosterName, pkgName)
				}
				if !cb(pkgName) {
					return nil
				}
			}
		}
	}
	return nil
}

// WalkPackageMeta walks all packages.
// if callback returns false, it will stop walking.
func (r *Roster) WalkPackageMeta(cb func(name string) bool) error {
	for _, rosterName := range []RosterName{ROSTER_CENTRAL} {
		entries, err := os.ReadDir(filepath.Join(r.metaDir, string(rosterName), "projects"))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				pkgName := entry.Name()
				if rosterName != ROSTER_CENTRAL {
					pkgName = fmt.Sprintf("%s/%s", rosterName, pkgName)
				}
				if !cb(pkgName) {
					return nil
				}
			}
		}
	}
	return nil
}

type SyncCheckStatus struct {
	RosterName   string
	SyncErr      error
	NeedSync     bool
	LocalCommit  plumbing.Hash
	RemoteCommit plumbing.Hash
}

func (r *Roster) SyncCheck() ([]*SyncCheckStatus, error) {
	ret := []*SyncCheckStatus{}
	for rosterName, rosterRepoUrl := range ROSTER_REPOS {
		repoPath := filepath.Join(r.metaDir, string(rosterName))
		if _, err := os.Stat(repoPath); err != nil {
			ret = append(ret, &SyncCheckStatus{
				RosterName: string(rosterName),
				SyncErr:    err,
				NeedSync:   true,
			})
			continue
		}
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			fmt.Printf("PlainOpen %T error:%s\n", err, err)
			return nil, err
		}
		headRef, err := repo.Head()
		if err != nil {
			ret = append(ret, &SyncCheckStatus{
				RosterName: string(rosterName),
				SyncErr:    err,
			})
			continue
		}
		remote := git.NewRemote(repo.Storer, &config.RemoteConfig{
			Name: string(git.DefaultRemoteName),
			URLs: []string{rosterRepoUrl},
		})
		remoteRefs, err := remote.List(&git.ListOptions{})
		if err != nil {
			ret = append(ret, &SyncCheckStatus{
				RosterName: string(rosterName),
				SyncErr:    err,
			})
			continue
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
		sc := &SyncCheckStatus{
			RosterName:   string(rosterName),
			LocalCommit:  headRef.Hash(),
			RemoteCommit: remoteRef.Hash(),
		}
		if headRef.Hash() != remoteRef.Hash() {
			sc.NeedSync = true
		}
		ret = append(ret, sc)
	}
	return ret, nil
}

func (r *Roster) SyncAll() error {
	for rosterName, rosterRepoUrl := range ROSTER_REPOS {
		if err := r.Sync(rosterName, rosterRepoUrl); err != nil {
			return err
		}
	}
	return nil
}

func (r *Roster) Sync(rosterName RosterName, rosterRepoUrl string) error {
	var repo *git.Repository
	var isBare = false
	repoPath := filepath.Join(r.metaDir, string(rosterName))
	if _, err := os.Stat(repoPath); err != nil {
		repo, err = git.PlainClone(repoPath, isBare, &git.CloneOptions{
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
		repo, err = git.PlainOpen(repoPath)
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

func (r *Roster) PushAllCache() error {
	for rosterName, rosterRepoUrl := range ROSTER_REPOS {
		if err := r.PushCache(rosterName, rosterRepoUrl); err != nil {
			return err
		}
	}
	return nil
}

func (r *Roster) PushCache(rosterName RosterName, rosterRepoUrl string) error {
	var repo *git.Repository
	repoPath := filepath.Join(r.metaDir, string(rosterName))
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree error: %w", err)
	}
	w.AddWithOptions(&git.AddOptions{All: true, Glob: ".cache/*"})
	w.Commit("rebuild-cache", &git.CommitOptions{})
	token := os.Getenv("GITHUB_TOKEN")
	err = repo.Push(&git.PushOptions{
		RemoteName: string(git.DefaultRemoteName),
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/heads/main:refs/heads/main"),
		},
		Auth: &http.BasicAuth{
			Username: "rebuild-cache",
			Password: token,
		},
	})
	if err != nil {
		return fmt.Errorf("pull error: %w", err)
	}
	return nil
}
