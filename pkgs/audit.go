package pkgs

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/machbase/neo-pkgdev/pkgs/elapsed"
)

func Audit(pathPackageYml string, output io.Writer) error {
	meta, err := parsePackageMetaFile(pathPackageYml)
	if err != nil {
		return err
	}
	if err := auditInjectRecipe(meta); err != nil {
		return err
	}
	fmt.Fprintln(output, ">> Distributable")
	fmt.Fprintln(output, "   ", "Github:", meta.Distributable.Github)
	fmt.Fprintln(output, "   ", "Url:", meta.Distributable.Url)
	fmt.Fprintln(output, "   ", "StripComponents:", meta.Distributable.StripComponents)
	if err := auditDescription(meta); err != nil {
		return err
	} else {
		fmt.Fprintln(output, ">> Description:")
		fmt.Fprintln(output, "   ", strings.Join(strings.Split(strings.TrimSpace(meta.Description), "\n"), "\n    "))
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
	} else {
		fmt.Fprintln(output, ">> Github")
		fmt.Fprintln(output, "   ", "Organization", repoInfo.Organization)
		fmt.Fprintln(output, "   ", "Repository", repoInfo.Repo)
	}

	if err := auditLicense(repoInfo); err != nil {
		return err
	} else {
		fmt.Fprintln(output, "   ", "License", repoInfo.License.SpdxId)
	}

	if err := auditDefaultBranch(repoInfo); err != nil {
		return err
	} else {
		fmt.Fprintln(output, "   ", "DefaultBranch", repoInfo.DefaultBranch)
	}

	latestInfo, err := GithubLatestReleaseInfo(httpClient, org, repo)
	if err != nil {
		return err
	}

	if err := auditLatestRelease(latestInfo); err != nil {
		return err
	} else {
		fmt.Fprintln(output, ">> LatestRelease:", latestInfo.Name)
		fmt.Fprintln(output, "   ", "tag:", latestInfo.TagName)
		fmt.Fprintln(output, "   ", "Published:", elapsed.LocalTime(latestInfo.PublishedAt, "en"))
	}
	return nil
}

func auditInjectRecipe(meta *PackageMeta) error {
	if meta.InjectRecipe.Type == "" {
		return fmt.Errorf("InjectRecipe.Type is empty")
	}
	if meta.InjectRecipe.Type != "web" {
		return fmt.Errorf("InjectRecipe.Type is invalid")
	}
	return nil
}

func auditDescription(meta *PackageMeta) error {
	desc := strings.TrimSpace(meta.Description)
	if desc == "" {
		return fmt.Errorf("description is empty")
	}
	return nil
}

func auditLicense(nfo *GhRepoInfo) error {
	if nfo.License.SpdxId == "" {
		return errors.New("license is not specified. (refer to https://spdx.org/licenses/)")
	}
	return nil
}

func auditDefaultBranch(nfo *GhRepoInfo) error {
	if nfo.DefaultBranch == "" {
		return errors.New("default branch is not specified")
	}
	return nil
}

func auditLatestRelease(nfo *GhReleaseInfo) error {
	if nfo.TagName == "" {
		return errors.New("latest release is not found")
	}
	_, err := semver.NewVersion(nfo.Name)
	if err != nil {
		return fmt.Errorf("latest release name is not a valid semver: %s (refer to https://semver.org/)", nfo.TagName)
	}
	return nil
}
