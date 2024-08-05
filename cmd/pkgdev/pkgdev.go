package pkgdev

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		Short: "Sync package index",
		RunE:  doSync,
	}
	syncCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	syncCmd.MarkPersistentFlagRequired("dir")

	searchCmd := &cobra.Command{
		Use:   "search [flags] <package name>",
		Short: "Search package info",
		RunE:  doSearch,
	}
	searchCmd.Args = cobra.ExactArgs(1)
	searchCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	searchCmd.MarkPersistentFlagRequired("dir")

	installCmd := &cobra.Command{
		Use:   "install [flags] <package name>",
		Short: "Install a package",
		RunE:  doInstall,
	}
	installCmd.Args = cobra.ExactArgs(1)
	installCmd.PersistentFlags().StringP("dir", "d", "", "`<BaseDir>` path to the package base directory")
	installCmd.MarkPersistentFlagRequired("dir")
	installCmd.PersistentFlags().StringP("version", "v", "latest", "`<Version>` of the package to install")

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

	rootCmd.AddCommand(
		syncCmd,
		searchCmd,
		installCmd,
		auditCmd,
		planCmd,
		buildCmd,
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
			fmt.Println("\nWhat your are looking for might be:")
			for _, s := range result.Possibles {
				if s.Github != nil {
					fmt.Printf("  %s   https://github.com/%s/%s installed: %s\n",
						s.Name, s.Github.Organization, s.Github.Name, s.InstalledVersion)
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
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}
	err = mgr.Sync()
	if err != nil {
		return err
	}
	return nil
}

func doInstall(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	version, err := cmd.Flags().GetString("version")
	if err != nil {
		return err
	}
	if version == "" {
		version = "latest"
	}
	mgr, err := pkgs.NewPkgManager(baseDir)
	if err != nil {
		return err
	}

	cache, err := mgr.Install(args[0], os.Stdout)
	if err != nil {
		return err
	}
	fmt.Println("Installed to", cache.InstalledPath, cache.InstalledVersion)
	return err
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

	files := []string{}
	for _, pkgName := range args {
		path := filepath.Join("projects", pkgName, "package.yml")
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
		fmt.Println("---->", pathPackageYml)
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
	var writer io.Writer
	if ghOut := os.Getenv("GITHUB_OUTPUT"); ghOut != "" {
		f, _ := os.OpenFile(ghOut, os.O_CREATE|os.O_WRONLY, 0644)
		defer f.Close()
		writer = f
	} else {
		writer = os.Stdout
	}

	installDest := cmd.Flag("install").Value.String()

	pathPackageYml := args[0]
	pkgPath := os.Getenv("PKGS_PATH")

	if pkgPath != "" && !strings.HasSuffix(pathPackageYml, "package.yml") && !strings.HasSuffix(pathPackageYml, "package.yaml") {
		pathPackageYml = filepath.Join(pkgPath, "projects", pathPackageYml, "package.yml")
		fmt.Println("---->", pathPackageYml)
	}
	if _, err := os.Stat(pathPackageYml); err != nil {
		return err
	}

	if err := pkgs.Build(pathPackageYml, installDest, writer); err != nil {
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
