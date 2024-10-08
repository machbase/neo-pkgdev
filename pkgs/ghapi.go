package pkgs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func GithubSplitPath(path string) (string, string, error) {
	toks := strings.Split(path, "/")
	if len(toks) != 2 {
		return "", "", fmt.Errorf("invalid github path: %s", path)
	}
	return toks[0], toks[1], nil
}

func GithubRepoInfo(client *http.Client, org, repo string) (*GhRepoInfo, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s", org, repo)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d\nURL: %s\n%s", rsp.StatusCode, endpoint, string(body))
	}
	ghRepo := &GhRepoInfo{Organization: strings.ToLower(org), Repo: strings.ToLower(repo)}
	json.Unmarshal(body, ghRepo)
	return ghRepo, nil
}

type GhRepoInfo struct {
	Organization    string     `json:"organization" yaml:"organization"`
	Repo            string     `json:"repo" yaml:"repo"`
	Name            string     `json:"name" yaml:"name"`
	FullName        string     `json:"full_name" yaml:"full_name"`
	Owner           *GhOwner   `json:"owner" yaml:"owner"`
	Private         bool       `json:"private" yaml:"private"`
	Description     string     `json:"description" yaml:"description"`
	Homepage        string     `json:"homepage" yaml:"homepage"`
	ForkCount       int        `json:"forks_count" yaml:"forks_count"`
	Forks           int        `json:"forks" yaml:"forks"`
	StargazersCount int        `json:"stargazers_count" yaml:"stargazers_count"`
	Language        string     `json:"language" yaml:"language"`
	License         *GhLicense `json:"license" yaml:"license"`
	DefaultBranch   string     `json:"default_branch" yaml:"default_branch"`
}

type GhLicense struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	SpdxId string `json:"spdx_id"`
	Url    string `json:"url"`
	NodeId string `json:"node_id"`
}

type GhOwner struct {
	Login            string `json:"login"`
	Id               int    `json:"id"`
	NodeId           string `json:"node_id"`
	AvatarUrl        string `json:"avatar_url"`
	GravatarId       string `json:"gravatar_id"`
	Url              string `json:"url"`
	HtmlUrl          string `json:"html_url"`
	SubscriptionsUrl string `json:"subscriptions_url"`
	OrganizationsUrl string `json:"organizations_url"`
	Type             string `json:"type"`
	SiteAdmin        bool   `json:"site_admin"`
}

func GithubReleaseInfo(client *http.Client, org, repo, ver string) (*GhReleaseInfo, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", org, repo, ver)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d\nURL: %s\n%s", rsp.StatusCode, endpoint, string(body))
	}
	ghRelease := &GhReleaseInfo{Organization: strings.ToLower(org), Repo: strings.ToLower(repo)}
	if err := ghRelease.Unmarshal(body); err != nil {
		return nil, err
	}
	return ghRelease, nil
}

func GithubLatestReleaseInfo(client *http.Client, org, repo string) (*GhReleaseInfo, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", org, repo)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d\nURL: %s\n%s", rsp.StatusCode, endpoint, string(body))
	}
	ghRelease := &GhReleaseInfo{Organization: strings.ToLower(org), Repo: strings.ToLower(repo)}
	if err := ghRelease.Unmarshal(body); err != nil {
		return nil, err
	}
	return ghRelease, nil
}

type GhReleaseInfo struct {
	Organization string    `json:"organization"`
	Repo         string    `json:"repo"`
	Name         string    `json:"name"`
	TagName      string    `json:"tag_name"`
	PublishedAt  time.Time `json:"published_at"`
	HtmlUrl      string    `json:"html_url"`
	TarballUrl   string    `json:"tarball_url"`
	Prerelease   bool      `json:"prerelease"`
}

func (ghRel *GhReleaseInfo) Unmarshal(body []byte) error {
	ghTimeFormat := "2006-01-02T15:04:05Z" //"2024-07-29T05:17:51Z"
	data := map[string]any{}
	if err := json.Unmarshal(body, &data); err != nil {
		return err
	}

	ghRel.Name = data["name"].(string)
	ghRel.TagName = data["tag_name"].(string)
	if t, err := time.Parse(ghTimeFormat, data["published_at"].(string)); err != nil {
		return err
	} else {
		ghRel.PublishedAt = t
	}
	ghRel.HtmlUrl = data["html_url"].(string)
	ghRel.TarballUrl = data["tarball_url"].(string)
	ghRel.Prerelease = data["prerelease"].(bool)
	return nil
}
