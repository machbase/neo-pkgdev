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

	syncCmd := &cobra.Command{
		Use:   "sync [flags]",
		Short: "Sync package roster",
		RunE:  doSync,
	}
	syncCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	syncCmd.MarkPersistentFlagRequired("dir")
	syncCmd.PersistentFlags().Bool("check", false, "Check updates only")

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

	upgradeCmd := &cobra.Command{
		Use:   "upgrade [flags] <package name, ...>",
		Short: "Upgrade packages",
		RunE:  doUpgrade,
	}
	upgradeCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	upgradeCmd.MarkPersistentFlagRequired("dir")

	installCmd := &cobra.Command{
		Use:   "install [flags] <package name>",
		Short: "Install a package",
		RunE:  doInstall,
	}
	installCmd.Args = cobra.ExactArgs(1)
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

	rootCmd.AddCommand(
		updateCmd,
		upgradeCmd,
		installCmd,
		uninstallCmd,
		searchCmd,
		syncCmd,
		auditCmd,
		planCmd,
		buildCmd,
		rebuildPlanCmd,
	)
	return rootCmd
}

func doSearch(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}
	name := args[0]
	result, err := mgr.Search(name, 10)
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
					if len(s.Github.Organization)+len(s.Github.Name)+len("https://github.com/")+1 > addrLen {
						addrLen = len(s.Github.Organization) + len(s.Github.Name) + len("https://github.com") + 1
					}
				}
			}
			for _, s := range result.Possibles {
				if s.Github != nil {
					addr := fmt.Sprintf("https://github.com/%s/%s", s.Github.Organization, s.Github.Name)
					if s.InstalledVersion == "" {
						fmt.Printf("  %-*s %-*s  -\n",
							nameLen, s.Name, addrLen, addr)
					} else {
						fmt.Printf("  %-*s %-*s   installed: %s\n",
							nameLen, s.Name, addrLen, addr, s.InstalledVersion)
					}
				}
			}
		}
	}
	return nil
}

func doSync(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return err
	}

	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}
	if check {
		check, err := mgr.SyncCheck()
		if err != nil {
			return err
		}
		if check != nil && check.NeedSync {
			fmt.Println("Need to sync")
		} else {
			fmt.Println("Already up-to-date")
		}
	} else {
		err = mgr.Sync()
		if err != nil {
			return err
		}
	}
	return nil
}

func doUpdate(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}
	upd, err := mgr.Update()
	if err != nil {
		return err
	}
	if upd == nil || len(upd.Updated) == 0 && len(upd.Upgradable) == 0 {
		fmt.Println("Already up-to-date")
	} else {
		fmt.Println("Updated packages:")
		if len(upd.Updated) > 0 {
			for _, p := range upd.Updated {
				fmt.Println("  ", p.PkgName, "updated", p.LatestRelease)
			}
		} else {
			fmt.Println("   no updated packages")
		}
		fmt.Println("Upgradable packages:")
		if len(upd.Upgradable) > 0 {
			for _, p := range upd.Upgradable {
				fmt.Println("  ", p.PkgName, p.InstalledVersion, "-->", p.LatestRelease, "available")
			}
		} else {
			fmt.Println("   no upgradable packages")
		}
	}
	return nil
}

func doUpgrade(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}
	results := mgr.Upgrade(args, nil)
	for _, r := range results {
		fmt.Println(r.PkgName, "upgraded", r.Cache.InstalledVersion, r.Cache.InstalledPath)
	}
	return nil
}

func doInstall(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}
	cache, err := mgr.Install(args[0], os.Stdout, nil)
	if err != nil {
		return err
	}
	fmt.Println("Installed to", cache.InstalledPath, cache.InstalledVersion)
	return err
}

func doUninstall(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}

	_, err = mgr.Uninstall(args[0], os.Stdout, nil)
	if err != nil {
		return err
	}
	fmt.Println("Uninstalled", args[0])
	return err
}

func doRebuildPlan(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}
	if err := mgr.Sync(); err != nil {
		return err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(20) * time.Second,
	}

	targetPkgs := []string{}
	mgr.WalkPackages(func(name string) bool {
		meta, err := mgr.LoadPackageMeta(name)
		if err != nil {
			return true
		}
		cache, _ := mgr.LoadPackageCache(name, meta, true)
		if cache == nil {
			fmt.Println(name, "cache not found")
			return true
		}
		dist, _ := cache.RemoteDistribution()
		rsp, err := httpClient.Head(dist.Url)
		if err != nil {
			fmt.Println(cache.Name, cache.LatestRelease, dist.Url, err.Error())
			return true
		}
		rsp.Body.Close()
		if rsp.StatusCode == 200 {
			contentLength := ""
			if cl := rsp.Header.Get("Content-Length"); cl != "" {
				contentLength = rsp.Header.Get("Content-Length")
			}
			fmt.Println(cache.Name, cache.LatestRelease, dist.Url, rsp.StatusCode, contentLength)
			return true
		}
		fmt.Println(cache.Name, cache.LatestRelease, dist.Url, rsp.StatusCode)
		packageYmlPath := filepath.Join(baseDir, "meta", string(pkgs.ROSTER_CENTRAL), "projects", name, "package.yml")
		targetPkgs = append(targetPkgs, packageYmlPath)
		return true
	})
	// even there are no packages to rebuild, it will print the plan
	// so that github action can see the plan is empty
	var writer io.Writer
	if ghOut := os.Getenv("GITHUB_OUTPUT"); ghOut != "" {
		f, _ := os.OpenFile(ghOut, os.O_CREATE|os.O_WRONLY, 0644)
		defer f.Close()
		writer = f
	} else {
		writer = os.Stdout
	}
	if err := pkgs.Plan(targetPkgs, writer); err != nil {
		return err
	}
	return nil
}

func doPlan(cmd *cobra.Command, args []string) error {
	var writer io.Writer
	if ghOut := os.Getenv("GITHUB_OUTPUT"); ghOut != "" {
		f, _ := os.OpenFile(ghOut, os.O_CREATE|os.O_WRONLY, 0644)
		defer f.Close()
		writer = f
	} else {
		writer = os.Stdout
	}

	pkgPath := os.Getenv("PKGS_PATH")
	files := []string{}
	for _, pkgName := range args {
		path := filepath.Join(pkgPath, "projects", pkgName, "package.yml")
		files = append(files, path)
		if _, err := os.Stat(path); err != nil {
			return err
		}
	}

	if err := pkgs.Plan(files, writer); err != nil {
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
	if err := pkgs.Audit(pathPackageYml, os.Stdout); err != nil {
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
	if err := pkgs.Build(pathPackageYml, installDest, os.Stdout); err != nil {
		return err
	}
	return nil
}

func print(nr *pkgs.PackageCache) {
	fmt.Println("Package             ", nr.Name)
	fmt.Println("Github              ", nr.Github)
	fmt.Println("Latest Release      ", nr.LatestRelease)
	fmt.Println("Latest Release Tag  ", nr.LatestReleaseTag)
	fmt.Println("Published At        ", nr.PublishedAt)
	fmt.Println("Url                 ", nr.Url)
	fmt.Println("StripComponents     ", nr.StripComponents)
	fmt.Println("Cached At           ", nr.CachedAt)
	fmt.Println("Installed Version   ", nr.InstalledVersion)
	fmt.Println("Installed Path      ", nr.InstalledPath)
}
