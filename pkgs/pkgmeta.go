package pkgs

import (
	"os"

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

func parsePackageMetaFile(path string) (*PackageMeta, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ret := &PackageMeta{}
	if err := yaml.Unmarshal(content, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
