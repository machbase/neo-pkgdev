package pkgdev

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/machbase/neo-pkgdev/pkgs"
	"github.com/machbase/neo-pkgdev/pkgs/builder"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cobra.EnableCommandSorting = false

	rootCmd := &cobra.Command{
		Use:           "neopkg [command] [flags] [args]",
		Short:         "neopkg is a package manager for machbase-neo",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Print(cmd.UsageString())
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search [flags] <package name>",
		Short: "Search package info",
		RunE:  doSearch,
	}
	searchCmd.Args = cobra.ExactArgs(1)
	searchCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	searchCmd.MarkPersistentFlagRequired("dir")

	updateCmd := &cobra.Command{
		Use:   "update [flags]",
		Short: "Update a package roster",
		RunE:  doUpdate,
	}
	updateCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	updateCmd.MarkPersistentFlagRequired("dir")

	installCmd := &cobra.Command{
		Use:   "install [flags] <package name, ...>",
		Short: "install packages",
		RunE:  doInstall,
	}
	installCmd.Args = cobra.MinimumNArgs(1)
	installCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	installCmd.MarkPersistentFlagRequired("dir")

	uninstallCmd := &cobra.Command{
		Use:   "uninstall [flags] <package name>",
		Short: "Uninstall a package",
		RunE:  doUninstall,
	}
	uninstallCmd.Args = cobra.ExactArgs(1)
	uninstallCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	uninstallCmd.MarkPersistentFlagRequired("dir")

	auditCmd := &cobra.Command{
		Use:   "audit [flags] <path to package.yml>",
		Short: "Audit a package",
		RunE:  doAudit,
	}
	auditCmd.Args = cobra.ExactArgs(1)

	planCmd := &cobra.Command{
		Use:   "plan [flags] <path to package.yml>",
		Short: "Planning to build a package",
		RunE:  doPlan,
	}
	planCmd.Args = cobra.MinimumNArgs(1)

	buildCmd := &cobra.Command{
		Use:   "build [flags] <path to package.yml>",
		Short: "Build a package",
		RunE:  doBuild,
	}
	buildCmd.Args = cobra.ExactArgs(1)
	buildCmd.PersistentFlags().String("install", "", "`<Dir>` path to install the package")

	rebuildPlanCmd := &cobra.Command{
		Use:   "rebuild-plan [flags]",
		Short: "Rebuild planning to build packages",
		RunE:  doRebuildPlan,
	}
	rebuildPlanCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	rebuildPlanCmd.MarkPersistentFlagRequired("dir")

	rebuildCacheCmd := &cobra.Command{
		Use:   "rebuild-cache [flags]",
		Short: "Rebuild cache",
		RunE:  doRebuildCache,
	}
	rebuildCacheCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	rebuildCacheCmd.MarkPersistentFlagRequired("dir")

	rootCmd.AddCommand(
		updateCmd,
		installCmd,
		uninstallCmd,
		searchCmd,
		auditCmd,
		planCmd,
		buildCmd,
		rebuildPlanCmd,
		rebuildCacheCmd,
	)
	return rootCmd
}

func doSearch(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	roster, err := pkgs.NewRoster(baseDir)
	if err != nil {
		return err
	}
	name := args[0]
	result, err := roster.Search(name, 10)
	if err != nil {
		return err
	}
	if result.ExactMatch != nil {
		print(result.ExactMatch)
	} else {
		fmt.Printf("Package %q not found\n", args[0])
		if len(result.Possibles) > 0 {
			fmt.Println("\nWhat you are looking for might be:")
			nameLen := 10
			addrLen := 10
			for _, s := range result.Possibles {
				if s.Github != nil {
					if len(s.Name) > nameLen {
						nameLen = len(s.Name)
					}
					if len(s.Github.FullName)+len("https://github.com/") > addrLen {
						addrLen = len(s.Github.FullName) + len("https://github.com") + 1
					}
				}
			}
			for _, s := range result.Possibles {
				if s.Github != nil {
					addr := fmt.Sprintf("https://github.com/%s", s.Github.FullName)
					inst, _ := roster.InstalledVersion(s.Name)
					if inst == nil {
						fmt.Printf("  %-*s %-*s  -\n",
							nameLen, s.Name, addrLen, addr)
					} else {
						fmt.Printf("  %-*s %-*s  installed: %s\n",
							nameLen, s.Name, addrLen, addr, inst.Version)
					}
				}
			}
		}
	}
	return nil
}

func doUpdate(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	roster, err := pkgs.NewRoster(baseDir)
	if err != nil {
		return err
	}
	upd, err := roster.Update()
	if err != nil {
		return err
	}
	if upd != nil && len(upd.Upgradable) > 0 {
		fmt.Println("Upgradable packages:")
		if len(upd.Upgradable) > 0 {
			for _, p := range upd.Upgradable {
				fmt.Println("  ", p.PkgName, p.InstalledVersion, "-->", strings.TrimPrefix(p.LatestRelease, "v"), "available")
			}
		} else {
			fmt.Println("   no upgradable packages")
		}
	}
	return nil
}

func doInstall(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	roster, err := pkgs.NewRoster(baseDir)
	if err != nil {
		return err
	}
	results := roster.Install(args, nil)
	for _, r := range results {
		if r.Installed == nil {
			continue
		}
		fmt.Println(r.PkgName, "installed", r.Installed.Version, r.Installed.Path)
	}
	return nil
}

func doUninstall(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	roster, err := pkgs.NewRoster(baseDir)
	if err != nil {
		return err
	}

	err = roster.Uninstall(args[0], os.Stdout, nil)
	if err != nil {
		return err
	}
	fmt.Println("Uninstalled", args[0])
	return err
}

func doRebuildCache(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}

	roster, err := pkgs.NewRoster(baseDir)
	if err != nil {
		return err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(20) * time.Second,
	}

	roster.WalkPackageMeta(func(name string) (ret bool) {
		ret = true
		meta, err := roster.LoadPackageMeta(name)
		if err != nil {
			fmt.Println(name, "meta load failed", err.Error())
			return
		}
		cache, err := roster.UpdatePackageCache(meta)
		if err != nil {
			fmt.Println(name, "cache update failed", err.Error())
			return
		}
		dist, err := cache.RemoteDistribution()
		if err != nil {
			fmt.Println(name, "distribution not found", err.Error())
			return
		}
		avail, err := dist.CheckAvailability(httpClient)
		if err != nil {
			fmt.Println(name, "distribution check failed", err.Error())
			return
		}
		if !avail.Available {
			fmt.Println(name, cache.LatestVersion, "distribution not available")
			return
		}
		fmt.Println(name, cache.LatestVersion, avail.DistUrl)
		if err := roster.WritePackageDistributionAvailability(avail); err != nil {
			fmt.Println(name, "distribution availability write failed", err.Error())
			return
		}
		return
	})

	roster.PushAllCache()
	return nil
}

func doRebuildPlan(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	roster, err := pkgs.NewRoster(baseDir)
	if err != nil {
		return err
	}
	if err := roster.SyncAll(); err != nil {
		return err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(20) * time.Second,
	}

	targetPkgs := []string{}
	roster.WalkPackageMeta(func(name string) (ret bool) {
		ret = true
		meta, err := roster.LoadPackageMeta(name)
		if err != nil {
			return
		}
		cache, _ := roster.UpdatePackageCache(meta)
		if cache == nil {
			fmt.Println(name, "cache not found")
			return
		}
		dist, err := cache.RemoteDistribution()
		if err != nil {
			fmt.Println(name, "distribution not found", err.Error())
			return
		}
		avail, err := dist.CheckAvailability(httpClient)
		if err != nil {
			fmt.Println(name, "distribution check failed", err.Error())
			return
		}
		fmt.Println(avail.String())

		roster.WritePackageDistributionAvailability(avail)

		if !avail.Available {
			packageYmlPath := filepath.Join(baseDir, "meta", string(pkgs.ROSTER_CENTRAL), "projects", name, "package.yml")
			targetPkgs = append(targetPkgs, packageYmlPath)
		}
		return
	})
	// even there are no packages to rebuild, it will print the plan
	// so that github action will not raise error
	var writer io.Writer
	if ghOut := os.Getenv("GITHUB_OUTPUT"); ghOut != "" {
		f, _ := os.OpenFile(ghOut, os.O_CREATE|os.O_WRONLY, 0644)
		defer f.Close()
		writer = f
	} else {
		writer = os.Stdout
	}
	// if targetPkgs is empty, it will intentionally pass the default example package
	// so that github action can see the plan is not empty, otherwise it will raise error
	if len(targetPkgs) == 0 {
		targetPkgs = []string{
			filepath.Join(baseDir, "meta", string(pkgs.ROSTER_CENTRAL), "projects", "neo-pkg-web-example", "package.yml"),
		}
	}
	if err := builder.Plan(targetPkgs, writer); err != nil {
		return err
	}
	return nil
}

func doPlan(cmd *cobra.Command, args []string) error {
	pkgPath := os.Getenv("PKGS_PATH")
	files := []string{}
	for _, pkgName := range args {
		path := filepath.Join(pkgPath, "projects", pkgName, "package.yml")
		if _, err := os.Stat(path); err != nil {
			fmt.Println("Package not found", path, err.Error())
			continue
		}
		if err := builder.Audit(path, os.Stdout); err != nil {
			fmt.Println("Audit failed", err.Error())
			continue
		}
		files = append(files, path)
	}

	var writer io.Writer
	if ghOut := os.Getenv("GITHUB_OUTPUT"); ghOut != "" {
		f, _ := os.OpenFile(ghOut, os.O_CREATE|os.O_WRONLY, 0644)
		defer f.Close()
		writer = f
	} else {
		writer = os.Stdout
	}
	if err := builder.Plan(files, writer); err != nil {
		return err
	}
	return nil
}

func doAudit(cmd *cobra.Command, args []string) error {
	pathPackageYml := args[0]
	pkgPath := os.Getenv("PKGS_PATH")

	if pkgPath != "" && !strings.HasSuffix(pathPackageYml, "package.yml") && !strings.HasSuffix(pathPackageYml, "package.yaml") {
		pathPackageYml = filepath.Join(pkgPath, "projects", pathPackageYml, "package.yml")
	}
	if _, err := os.Stat(pathPackageYml); err != nil {
		return err
	}
	if err := builder.Audit(pathPackageYml, os.Stdout); err != nil {
		return err
	}
	cmd.Print("Audit successful\n")
	return nil
}

func doBuild(cmd *cobra.Command, args []string) error {
	pathPackageYml := args[0]
	pkgPath := os.Getenv("PKGS_PATH")

	if pkgPath != "" && !strings.HasSuffix(pathPackageYml, "package.yml") && !strings.HasSuffix(pathPackageYml, "package.yaml") {
		pathPackageYml = filepath.Join(pkgPath, "projects", pathPackageYml, "package.yml")
	}
	if _, err := os.Stat(pathPackageYml); err != nil {
		return err
	}
	installDest := cmd.Flag("install").Value.String()
	if err := builder.Build(pathPackageYml, installDest, os.Stdout); err != nil {
		return err
	}
	return nil
}

func print(nr *pkgs.PackageCache) {
	fmt.Println("Package             ", nr.Name)
	if nr.Github != nil {
		fmt.Println("Organization        ", nr.Github.Organization)
		fmt.Println("Repository          ", nr.Github.Name)
		fmt.Println("Description         ", nr.Github.Description)
		fmt.Println("License             ", nr.Github.License)
	}
	fmt.Println("Latest Version      ", nr.LatestVersion)
	fmt.Println("Latest Release      ", nr.LatestRelease)
	fmt.Println("Latest Release Tag  ", nr.LatestReleaseTag)
	fmt.Println("Published At        ", nr.PublishedAt)
	if nr.Url != "" {
		fmt.Println("Url                 ", nr.Url)
	}
	fmt.Println("StripComponents     ", nr.StripComponents)
}
