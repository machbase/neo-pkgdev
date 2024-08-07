package pkgs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/machbase/neo-pkgdev/pkgs"
)

func TestBuild(t *testing.T) {
	t.Skip("Skip test")
	builder, err := pkgs.NewBuilder(nil, "0.0.1",
		pkgs.WithWorkDir("./tmp/builder"),
		pkgs.WithDistDir("./tmp/dist"),
	)
	if err != nil {
		panic(err)
	}
	err = builder.Build("latest")
	if err != nil {
		panic(err)
	}
	fmt.Println("Build successful")
	// Output:
	// &{Orgnization:machbase Repo:neo-pkg-web-example Name:v0.0.1 TagName:v0.0.1 PublishedAt:2024-07-29 05:17:51 +0000 UTC HtmlUrl:https://github.com/machbase/neo-pkg-web-example/releases/tag/v0.0.1 TarballUrl:https://api.github.com/repos/machbase/neo-pkg-web-example/tarball/v0.0.1 Prerelease:false}
	// Build successful
}

func TestDeploy(t *testing.T) {
	s3_key_id := os.Getenv("AWS_ACCESS_KEY_ID")
	s3_secret_key := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if s3_key_id == "" || s3_secret_key == "" {
		t.Log("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set")
		t.Skip("Skip deploy test")
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-northeast-2"),
		Credentials: credentials.NewStaticCredentials(s3_key_id, s3_secret_key, ""),
	})
	if err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}
	org := "machbase"
	repo := "neo-pkg-web-example"
	bucket := "p-edge-packages"
	archivePath := "../tmp/neo-pkg-web-example-0.0.5-darwin-arm64.tar.gz"
	file, err := os.Open(archivePath)
	if err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}
	defer file.Close()

	_, err = s3.New(sess).PutObject(&s3.PutObjectInput{
		Bucket:             aws.String(bucket),
		Key:                aws.String(fmt.Sprintf("neo-pkg/%s/%s/%s", org, repo, filepath.Base(archivePath))),
		Body:               file,
		ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}
}
