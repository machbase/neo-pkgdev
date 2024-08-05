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
		for _, platform := range meta.Platforms {
			var bp BuildPlatform
			pos, parch, ok := strings.Cut(platform, "/")
			if !ok {
				fmt.Printf("platform %q is invalid", platform)
				os.Exit(1)
			}
			switch strings.ToLower(pos) {
			case "linux":
				switch strings.ToLower(parch) {
				case "amd64":
					bp.OS = []string{"ubuntu-latest"}
					bp.Name = "linux+amd64"
					bp.Container = "ubuntu:22.04"
				case "arm64":
					bp.OS = []string{"ubuntu-latest"}
					bp.Name = "linux+arm64"
					bp.Container = "arm64v8/ubuntu:22.04"
				case "arm", "arm32", "armv7":
					bp.OS = []string{"ubuntu-latest"}
					bp.Name = "linux+arm"
					bp.Container = "armv7/armhf-ubuntu"
				default:
					fmt.Printf("platform %q is invalid", platform)
					os.Exit(1)
				}
			case "darwin":
				switch strings.ToLower(parch) {
				case "arm64":
					bp.OS = []string{"macos-latest"}
					bp.Name = "macos+arm64"
				case "amd64":
					bp.OS = []string{"macos-13"}
					bp.Name = "macos+amd64"
				default:
					fmt.Printf("platform %q is invalid", platform)
					os.Exit(1)
				}
			case "windows":
				switch strings.ToLower(parch) {
				case "amd64":
					bp.OS = []string{"windows-latest"}
					bp.Name = "windows+amd64"
				default:
					fmt.Printf("platform %q is invalid", platform)
					os.Exit(1)
				}
			default:
				fmt.Printf("platform %q is invalid", platform)
				os.Exit(1)
			}
			plans = append(plans, &BuildPlan{Platform: bp, Pkg: pkgName})
		}
		// platform independent
		if len(meta.Platforms) == 0 {
			bp := BuildPlatform{
				OS:        []string{"ubuntu-latest"},
				Name:      "linux+noarch",
				Container: "ubuntu:22.04",
			}
			plans = append(plans, &BuildPlan{Platform: bp, Pkg: pkgName})
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
	Pkg      string        `json:"pkg"` // package name
}

type BuildPlatform struct {
	OS        []string `json:"os"`
	Name      string   `json:"name"`
	Container string   `json:"container,omitempty"`
}
