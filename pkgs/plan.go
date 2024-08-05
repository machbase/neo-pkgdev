package pkgs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Plan(pkgFiles []string, output io.Writer) error {
	plans := []*BuildPlan{}
	for _, pkgPath := range pkgFiles {
		meta, err := parsePackageMetaFile(pkgPath)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		pkgName := filepath.Base(filepath.Dir(pkgPath))
		if meta.InjectRecipe.Type == "web" {
			plan := &BuildPlan{
				Platform: BuildPlatform{
					OS:        []string{"ubuntu-latest"},
					Name:      "linux+x86-64",
					Container: "debian:buster-slim",
					TinyName:  "*nix64",
				},
				Pkg: pkgName,
			}
			plans = append(plans, plan)
		} else {
			fmt.Println("package does not support web injection")
			os.Exit(1)
		}
	}
	sb := &strings.Builder{}
	if err := json.NewEncoder(sb).Encode(plans); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(output, "matrix=%s", sb.String())
	return nil
}

type BuildPlan struct {
	Platform BuildPlatform `json:"platform"`
	Pkg      string        `json:"pkg"`
}

type BuildPlatform struct {
	OS        []string `json:"os"`
	Name      string   `json:"name"`
	Container string   `json:"container,omitempty"`
	TinyName  string   `json:"tinyname,omitempty"`
}
