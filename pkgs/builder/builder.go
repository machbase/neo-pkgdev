package builder

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/machbase/neo-pkgdev/pkgs"
)

func Build(pathPackageYml string, dest string, output io.Writer) error {
	meta, err := pkgs.LoadPackageMetaFile(pathPackageYml)
	if err != nil {
		return err
	}

	if meta.Distributable.Url != "" {
		fmt.Fprintln(output, "Distribution URL:", meta.Distributable.Url)
		fmt.Fprintln(output, "Skip Build.")
		return nil
	}

	org, repo, err := pkgs.GithubSplitPath(meta.Distributable.Github)
	if err != nil {
		return err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		Timeout: time.Duration(10) * time.Second,
	}
	repoInfo, err := pkgs.GithubRepoInfo(httpClient, org, repo)
	if err != nil {
		return err
	}

	latestInfo, err := pkgs.GithubLatestReleaseInfo(httpClient, org, repo)
	if err != nil {
		return err
	}

	var versionName = strings.TrimPrefix(latestInfo.Name, "v")
	versionName = strings.TrimPrefix(versionName, "V")
	if meta.PackageName() == "neo-pkg-web-example" {
		rsp, err := httpClient.Head(fmt.Sprintf("https://p-edge-packages.s3.ap-northeast-2.amazonaws.com/neo-pkg/machbase/neo-pkg-web-example/neo-pkg-web-example-%s.tar.gz", versionName))
		if err == nil && rsp.StatusCode == 200 {
			fmt.Fprintln(output, "Skip Build.")
			return nil
		}
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
	var buildRun string
	if len(meta.BuildRecipe.Scripts) > 1 {
		for _, script := range meta.BuildRecipe.Scripts {
			if script.Platform == "" {
				buildRun = script.Run
				continue
			}
			if script.Platform == runtime.GOOS {
				buildRun = script.Run
				break
			}
		}
	} else {
		buildRun = meta.BuildRecipe.Scripts[0].Run
	}

	if runtime.GOOS == "windows" {
		// Windows build script
		var buildScript string
		if f, err := pkgs.MakeScriptFile([]string{buildRun}, dest, "__build__.cmd"); err != nil {
			return err
		} else {
			buildScript = f
		}
		buildCmd := exec.Command("cmd", "/c", buildScript)
		buildCmd.Dir = dest
		buildCmd.Env = append(os.Environ(), meta.BuildRecipe.Env...)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return err
		}
	} else {
		// Create build script
		var buildScript string
		if f, err := pkgs.MakeScriptFile([]string{buildRun}, dest, "__build__.sh"); err != nil {
			return err
		} else {
			buildScript = f
		}
		// Run build script
		buildCmd := exec.Command("sh", "-c", buildScript)
		buildCmd.Dir = dest
		buildCmd.Env = append(os.Environ(), meta.BuildRecipe.Env...)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return err
		}
	}
	// Test the built files
	if meta.TestRecipe != nil {
		if runtime.GOOS == "windows" {
			for _, line := range meta.TestRecipe.Script {
				// Windows test script
				buildCmd := exec.Command("cmd", "/c", line)
				buildCmd.Dir = dest
				buildCmd.Env = append(os.Environ(), meta.BuildRecipe.Env...)
				buildCmd.Stdout = os.Stdout
				buildCmd.Stderr = os.Stderr
				if err := buildCmd.Run(); err != nil {
					return err
				}
			}
		} else {
			var testScript string
			if f, err := pkgs.MakeScriptFile(meta.TestRecipe.Script, dest, "__test__.sh"); err != nil {
				return err
			} else {
				testScript = f
			}
			var testCmd *exec.Cmd
			if runtime.GOOS == "windows" {
				testCmd = exec.Command("cmd", "/c", testScript)
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
	}
	// Copy the built files to dist dir
	var archiveCmd *exec.Cmd
	var archivePath = fmt.Sprintf("%s-%s%s", repoInfo.Repo, versionName, archiveExt)
	if len(meta.Platforms) >= 1 {
		archivePath = fmt.Sprintf("%s-%s-%s-%s%s", repoInfo.Repo, versionName, runtime.GOOS, runtime.GOARCH, archiveExt)
	}
	if runtime.GOOS == "windows" {
		args := []string{"-c", "Compress-Archive", "-DestinationPath", archivePath, "-Path", strings.Join(meta.Provides, ",")}
		fmt.Println("Debug", args)
		archiveCmd = exec.Command("powershell", args...)
	} else {
		args := strings.Join([]string{"tar", "czf", archivePath, strings.Join(meta.Provides, " ")}, " ")
		fmt.Println("Debug", args)
		archiveCmd = exec.Command("sh", "-c", args)
	}
	archiveCmd.Dir = dest
	archiveCmd.Stdout = os.Stdout
	archiveCmd.Stderr = os.Stderr
	if err := archiveCmd.Run(); err != nil {
		return err
	}
	fmt.Fprintf(output, "Built %s\n", archivePath)

	// Deploy the built files to S3
	s3_key_id := os.Getenv("AWS_ACCESS_KEY_ID")
	s3_secret_key := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if s3_key_id != "" && s3_secret_key != "" {
		file, err := os.Open(filepath.Join(dest, archivePath))
		if err != nil {
			return err
		}
		hmx := sha256.New()
		if _, err := io.Copy(hmx, file); err != nil {
			return err
		}
		file.Close()
		checksum := base64.StdEncoding.EncodeToString(hmx.Sum(nil))

		file, err = os.Open(filepath.Join(dest, archivePath))
		if err != nil {
			return err
		}
		defer file.Close()

		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("ap-northeast-2"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3_key_id, s3_secret_key, "")),
		)
		if err != nil {
			return err
		}
		client := s3.NewFromConfig(cfg)
		_, err = client.PutObject(context.TODO(),
			&s3.PutObjectInput{
				Bucket:         aws.String("p-edge-packages"),
				Key:            aws.String(fmt.Sprintf("neo-pkg/%s/%s/%s", org, repo, filepath.Base(archivePath))),
				Body:           file,
				ChecksumSHA256: aws.String(checksum),
			})
		if err != nil {
			return err
		}

		fmt.Fprintln(output, "Deployed. sha-256:", checksum)
	} else {
		fmt.Fprintln(output, "Skip deploy.")
	}
	return nil
}

/*
type Builder struct {
	ds *pkgs.PackageMeta

	workDir    string
	distDir    string
	httpClient *http.Client
}

func NewBuilder(meta *pkgs.PackageMeta, version string, opts ...BuilderOption) (*Builder, error) {
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
	var targetRelease *pkgs.GhReleaseInfo
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

func (b *Builder) getReleaseInfo(ver string) (*pkgs.GhReleaseInfo, error) {
	if b.ds.Distributable.Github == "" {
		return nil, fmt.Errorf("distributable.github is not set")
	}
	org, repo, err := pkgs.GithubSplitPath(b.ds.Distributable.Github)
	if err != nil {
		return nil, err
	}
	// github's default distribution, strip_components is 1
	if b.ds.Distributable.StripComponents == 0 {
		b.ds.Distributable.StripComponents = 1
	}

	if ver != "" || strings.ToLower(ver) == "latest" {
		return pkgs.GithubLatestReleaseInfo(b.httpClient, org, repo)
	} else {
		return pkgs.GithubReleaseInfo(b.httpClient, org, repo, ver)
	}
}
*/
