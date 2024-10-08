package builder_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/machbase/neo-pkgdev/pkgs"
)

func TestLoadMeta(t *testing.T) {
	for _, path := range []string{"./testdata/test1.yml", "./testdata/test2.yml"} {
		_, err := pkgs.LoadPackageMetaFile(path)
		if err != nil {
			t.Log(path, err.Error())
			t.Fail()
			return
		}
		t.Log("ok", path)
	}
}

func TestDeploy(t *testing.T) {
	t.Skip("Skip deploy test")
	s3_key_id := os.Getenv("AWS_ACCESS_KEY_ID")
	s3_secret_key := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if s3_key_id == "" || s3_secret_key == "" {
		t.Log("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set")
		t.Skip("Skip deploy test")
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
	hmx := sha256.New()
	if _, err := io.Copy(hmx, file); err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}
	file.Close()
	checksum := base64.StdEncoding.EncodeToString(hmx.Sum(nil))

	file, err = os.Open(archivePath)
	if err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}
	defer file.Close()

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("ap-northeast-2"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3_key_id, s3_secret_key, "")),
	)
	if err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}
	client := s3.NewFromConfig(cfg)
	_, err = client.PutObject(context.TODO(),
		&s3.PutObjectInput{
			Bucket:         aws.String(bucket),
			Key:            aws.String(fmt.Sprintf("neo-pkg/%s/%s/%s", org, repo, filepath.Base(archivePath))),
			Body:           file,
			ChecksumSHA256: aws.String(checksum),
		})
	if err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}

	t.Log("Deployed. sha-256:", checksum)
}
