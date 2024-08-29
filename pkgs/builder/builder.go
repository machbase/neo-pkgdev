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
	"github.com/machbase/neo-pkgdev/pkgs/tar"
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
	srcTarBall := fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s.tar.gz", org, repo, latestInfo.TagName)
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

	buildRun := pkgs.FindScript(meta.BuildRecipe.Scripts, runtime.GOOS)

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
		testRun := pkgs.FindScript(meta.TestRecipe.Scripts, runtime.GOOS)

		if runtime.GOOS == "windows" {
			var testScript string
			if f, err := pkgs.MakeScriptFile([]string{testRun}, dest, "__test__.cmd"); err != nil {
				return err
			} else {
				testScript = f
			}
			// Windows test script
			testCmd := exec.Command("cmd", "/c", testScript)
			testCmd.Dir = dest
			testCmd.Env = append(os.Environ(), meta.BuildRecipe.Env...)
			testCmd.Stdout = os.Stdout
			testCmd.Stderr = os.Stderr
			if err := testCmd.Run(); err != nil {
				return err
			}
		} else {
			var testScript string
			if f, err := pkgs.MakeScriptFile([]string{testRun}, dest, "__test__.sh"); err != nil {
				return err
			} else {
				testScript = f
			}
			testCmd := exec.Command("sh", "-c", testScript)
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
	var archivePath = fmt.Sprintf("%s-%s.tar.gz", repoInfo.Repo, versionName)
	if len(meta.Platforms) >= 1 {
		archivePath = fmt.Sprintf("%s-%s-%s-%s.tar.gz", repoInfo.Repo, versionName, runtime.GOOS, runtime.GOARCH)
	}
	if runtime.GOOS == "windows" {
		err := tar.Archive(archivePath, meta.Provides)
		if err != nil {
			return err
		}
	} else {
		args := strings.Join([]string{"tar", "czf", archivePath, strings.Join(meta.Provides, " ")}, " ")
		fmt.Println("Debug", args)
		archiveCmd := exec.Command("sh", "-c", args)
		archiveCmd.Dir = dest
		archiveCmd.Stdout = os.Stdout
		archiveCmd.Stderr = os.Stderr
		if err := archiveCmd.Run(); err != nil {
			return err
		}
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
