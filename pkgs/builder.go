package pkgs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func Build(pathPackageYml string, dest string, output io.Writer) error {
	meta, err := parsePackageMetaFile(pathPackageYml)
	if err != nil {
		return err
	}

	if meta.Distributable.Url != "" {
		fmt.Fprintln(output, "Distribution URL:", meta.Distributable.Url)
		fmt.Fprintln(output, "Skip Build.")
		return nil
	}

	org, repo, err := GithubSplitPath(meta.Distributable.Github)
	if err != nil {
		return err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(10) * time.Second,
	}
	repoInfo, err := GithubRepoInfo(httpClient, org, repo)
	if err != nil {
		return err
	}

	latestInfo, err := GithubLatestReleaseInfo(httpClient, org, repo)
	if err != nil {
		return err
	}
	fmt.Fprintln(output, "Build", repoInfo.Organization, repoInfo.Repo, latestInfo.TagName)

	// Download the source tarball
	var wgetCmd *exec.Cmd
	if dest == "" {
		dest = "./tmp"
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	var archiveExt = ".tar.gz"
	if runtime.GOOS == "windows" {
		archiveExt = ".zip"
	}
	srcTarBall := fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s%s", org, repo, latestInfo.TagName, archiveExt)
	if runtime.GOOS == "windows" {
		wgetCmd = exec.Command("powershell", "-c", fmt.Sprintf("Invoke-WebRequest %s -OutFile %s\\src.zip", srcTarBall, dest))
	} else {
		wgetCmd = exec.Command("sh", "-c", fmt.Sprintf("wget %s -O %s/src.tar.gz", srcTarBall, dest))
	}
	wgetCmd.Stdout = os.Stdout
	wgetCmd.Stderr = os.Stderr
	if err := wgetCmd.Run(); err != nil {
		return err
	}
	// Extract the source tarball
	var tarCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		tarCmd = exec.Command("powershell", "-c", fmt.Sprintf("Expand-Archive %s\\src.zip -DestinationPath %s", dest, dest))
	} else {
		tarCmd = exec.Command("sh", "-c", fmt.Sprintf("tar xf %s/src.tar.gz --strip-components=1 -C %s", dest, dest))
	}
	tarCmd.Stdout = os.Stdout
	tarCmd.Stderr = os.Stderr
	if err := tarCmd.Run(); err != nil {
		return err
	}
	// Windows strip-components
	if runtime.GOOS == "windows" {
		mvCmd := exec.Command("powershell", "-c", fmt.Sprintf("Move-Item %s\\%s-%s\\* %s", dest, repo, strings.TrimPrefix(latestInfo.TagName, "v"), dest))
		mvCmd.Stdout = os.Stdout
		mvCmd.Stderr = os.Stderr
		if err := mvCmd.Run(); err != nil {
			return err
		}
	}
	// Create build script
	var buildScript string
	if f, err := makeScriptFile(meta.BuildRecipe.Script, dest, "__build__.sh"); err != nil {
		return err
	} else {
		buildScript = f
	}
	// Run build script
	var buildCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		buildCmd = exec.Command("powershell", "-c", buildScript)
	} else {
		buildCmd = exec.Command("sh", "-c", buildScript)
	}
	buildCmd.Dir = dest
	buildCmd.Env = append(os.Environ(), meta.BuildRecipe.Env...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return err
	}
	// Test the built files
	if meta.TestRecipe != nil {
		var testScript string
		if f, err := makeScriptFile(meta.TestRecipe.Script, dest, "__test__.sh"); err != nil {
			return err
		} else {
			testScript = f
		}
		var testCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			testCmd = exec.Command("powershell", "-c", testScript)
		} else {
			testCmd = exec.Command("sh", "-c", testScript)
		}
		testCmd.Dir = dest
		testCmd.Env = append(os.Environ(), meta.TestRecipe.Env...)
		testCmd.Stdout = os.Stdout
		testCmd.Stderr = os.Stderr
		if err := testCmd.Run(); err != nil {
			return err
		}
	}
	// Copy the built files to dist dir
	var archiveCmd *exec.Cmd
	var archivePath = fmt.Sprintf("%s-%s%s", repoInfo.Repo, latestInfo.TagName, archiveExt)
	if len(meta.Platforms) >= 1 {
		archivePath = fmt.Sprintf("%s-%s-%s-%s%s", repoInfo.Repo, latestInfo.TagName, runtime.GOOS, runtime.GOARCH, archiveExt)
	}
	if runtime.GOOS == "windows" {
		args := []string{"-c", "Compress-Archive", "-DestinationPath", archivePath, "-Path", strings.Join(meta.Provides, ",")}
		fmt.Println("Debug", args)
		archiveCmd = exec.Command("powershell", args...)
	} else {
		archiveCmd = exec.Command("sh", "-c", strings.Join([]string{"tar", "czf", archivePath, strings.Join(meta.Provides, " ")}, " "))
	}
	archiveCmd.Dir = dest
	archiveCmd.Stdout = os.Stdout
	archiveCmd.Stderr = os.Stderr
	if err := archiveCmd.Run(); err != nil {
		return err
	}
	return nil
}

func makeScriptFile(script []string, destDir string, filename string) (string, error) {
	var buildScript, _ = filepath.Abs(filepath.Join(destDir, filename))
	f, err := os.OpenFile(buildScript, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for _, line := range script {
		fmt.Fprintln(f, line)
	}
	return buildScript, nil
}

type Builder struct {
	ds *PackageMeta

	workDir    string
	distDir    string
	httpClient *http.Client
}

func NewBuilder(meta *PackageMeta, version string, opts ...BuilderOption) (*Builder, error) {
	ret := &Builder{ds: meta}
	for _, opt := range opts {
		opt(ret)
	}
	return ret, nil
}

type BuilderOption func(*Builder)

func WithWorkDir(workDir string) BuilderOption {
	return func(b *Builder) {
		b.workDir = workDir
	}
}

func WithDistDir(distDir string) BuilderOption {
	return func(b *Builder) {
		b.distDir = distDir
	}
}

// Build builds the package with the given version.
// if version is empty or "latest", it will use the latest version.
func (b *Builder) Build(ver string) error {
	if b.httpClient == nil {
		b.httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
			Timeout: time.Duration(10) * time.Second,
		}
	}
	var targetRelease *GhReleaseInfo
	if lr, err := b.getReleaseInfo(ver); err != nil {
		return err
	} else {
		targetRelease = lr
	}

	// mkdir workdir
	if err := os.MkdirAll(b.workDir, 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	// download the tarball
	srcTarBall := "src.tar.gz"
	cmd := exec.Command("sh", "-c", fmt.Sprintln("wget", targetRelease.TarballUrl, "-O", srcTarBall))
	cmd.Dir = b.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	// extract the tarball // tar xvf ./src.tar.gz --strip-components=1
	cmd = exec.Command("sh", "-c", fmt.Sprintln("tar", "xf", srcTarBall, "--strip-components", fmt.Sprintf("%d", b.ds.Distributable.StripComponents)))
	cmd.Dir = b.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	// make build script
	buildScript := "__build__.sh"
	f, err := os.OpenFile(filepath.Join(b.workDir, buildScript), os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, line := range b.ds.BuildRecipe.Script {
		fmt.Fprintln(f, line)
	}
	if err := f.Sync(); err != nil {
		return err
	}

	// run build script
	cmd = exec.Command("sh", "-c", "./"+buildScript)
	cmd.Env = append(os.Environ(), b.ds.BuildRecipe.Env...)
	cmd.Dir = b.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// mkdir workdir
	if err := os.MkdirAll(b.distDir, 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	// copy the built files to dist dir
	for _, pv := range b.ds.Provides {
		cmd = exec.Command("rsync", "-r", filepath.Join(b.workDir, pv), b.distDir)
		cmd.Env = append(os.Environ(), b.ds.BuildRecipe.Env...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// tar xvf ./src.tar.gz --strip-components=1
	fmt.Printf("%+v\n", targetRelease)
	return nil
}

func (b *Builder) getReleaseInfo(ver string) (*GhReleaseInfo, error) {
	if b.ds.Distributable.Github == "" {
		return nil, fmt.Errorf("distributable.github is not set")
	}
	org, repo, err := GithubSplitPath(b.ds.Distributable.Github)
	if err != nil {
		return nil, err
	}
	// github's default distribution, strip_components is 1
	if b.ds.Distributable.StripComponents == 0 {
		b.ds.Distributable.StripComponents = 1
	}

	if ver != "" || strings.ToLower(ver) == "latest" {
		return GithubLatestReleaseInfo(b.httpClient, org, repo)
	} else {
		return GithubReleaseInfo(b.httpClient, org, repo, ver)
	}
}
