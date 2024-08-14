package pkgs_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/machbase/neo-pkgdev/pkgs"
)

func ExampleGithubRepoInfo() {
	nfo, err := pkgs.GithubRepoInfo(http.DefaultClient, "machbase", "neo-pkg-web-example")
	if err != nil {
		panic(err)
	}
	if nfo == nil {
		panic("repo not found")
	}

	sb := &strings.Builder{}
	enc := json.NewEncoder(sb)
	enc.SetIndent("", "  ")
	enc.Encode(nfo)

	fmt.Println(sb.String())

	// Output:
	// {
	//   "organization": "machbase",
	//   "repo": "neo-pkg-web-example",
	//   "name": "neo-pkg-web-example",
	//   "full_name": "machbase/neo-pkg-web-example",
	//   "owner": {
	//     "login": "machbase",
	//     "id": 30223383,
	//     "node_id": "MDEyOk9yZ2FuaXphdGlvbjMwMjIzMzgz",
	//     "avatar_url": "https://avatars.githubusercontent.com/u/30223383?v=4",
	//     "gravatar_id": "",
	//     "url": "https://api.github.com/users/machbase",
	//     "html_url": "https://github.com/machbase",
	//     "subscriptions_url": "https://api.github.com/users/machbase/subscriptions",
	//     "organizations_url": "https://api.github.com/users/machbase/orgs",
	//     "type": "Organization",
	//     "site_admin": false
	//   },
	//   "private": false,
	//   "description": "neo package web application example",
	//   "homepage": "https://www.machbase.com",
	//   "forks_count": 1,
	//   "forks": 1,
	//   "stargazers_count": 1,
	//   "language": "TypeScript",
	//   "license": {
	//     "key": "apache-2.0",
	//     "name": "Apache License 2.0",
	//     "spdx_id": "Apache-2.0",
	//     "url": "https://api.github.com/licenses/apache-2.0",
	//     "node_id": "MDc6TGljZW5zZTI="
	//   },
	//   "default_branch": "main"
	// }
}
