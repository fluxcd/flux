package registry

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"encoding/base64"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/middleware"
	"io/ioutil"
	"os"
)

var (
	testTags = []string{testTagStr, "anotherTag"}
	mClient  = NewMockClient(
		func(repository Repository, tag string) (flux.Image, error) {
			return img, nil
		},
		func(repository Repository) ([]string, error) {
			return testTags, nil
		},
	)
)

func TestRegistry_GetRepository(t *testing.T) {
	fact := NewMockClientFactory(mClient, nil)
	reg := NewRegistry(fact, log.NewNopLogger())
	imgs, err := reg.GetRepository(testRepository)
	if err != nil {
		t.Fatal(err)
	}
	// Dev note, the tags will look the same because we are returning the same
	// Image from the mock remote. But they are distinct images.
	if len(testTags) != len(imgs) {
		t.Fatal("Expecting %v images, but got %v", len(testTags), len(imgs))
	}
}

func TestRegistry_GetRepositoryFactoryError(t *testing.T) {
	errFact := NewMockClientFactory(mClient, errors.New(""))
	reg := NewRegistry(errFact, nil)
	_, err := reg.GetRepository(testRepository)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_GetRepositoryManifestError(t *testing.T) {
	errClient := NewMockClient(
		func(repository Repository, tag string) (flux.Image, error) {
			return flux.Image{}, errors.New("")
		},
		func(repository Repository) ([]string, error) {
			return testTags, nil
		},
	)
	errFact := NewMockClientFactory(errClient, nil)
	reg := NewRegistry(errFact, log.NewNopLogger())
	_, err := reg.GetRepository(testRepository)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

// Note: This actually goes off to docker hub to find the Image.
// It will fail if there is not internet connection
func TestRemoteFactory_RawClient(t *testing.T) {
	// No credentials required for public Image
	fact := NewRemoteClientFactory(Credentials{}, log.NewNopLogger(), middleware.RateLimiterConfig{
		RPS:   200,
		Burst: 1,
	})
	img, err := flux.ParseImage("alpine:latest", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	testRepository = RepositoryFromImage(img)

	// Refresh tags first
	var tags []string
	client, err := fact.ClientFor(testRepository.Host())
	if err != nil {
		t.Fatal(err)
	}

	tags, err = client.Tags(testRepository)
	if err != nil {
		t.Fatal(err)
	}
	client.Cancel()
	if len(tags) == 0 {
		t.Fatal("Should have some tags")
	}

	client, err = fact.ClientFor(testRepository.Host())
	if err != nil {
		t.Fatal(err)
	}
	newImg, err := client.Manifest(testRepository, tags[0])
	if err != nil {
		t.Fatal(err)
	}
	if newImg.ID.String() == "" {
		t.Fatal("Should image ")
	}
	if newImg.CreatedAt.IsZero() {
		t.Fatal("CreatedAt time was 0")
	}
	client.Cancel()
}

func TestRemoteFactory_InvalidHost(t *testing.T) {
	fact := NewRemoteClientFactory(Credentials{}, log.NewNopLogger(), middleware.RateLimiterConfig{})
	img, err := flux.ParseImage("invalid.host/library/alpine:latest", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	testRepository = RepositoryFromImage(img)
	client, err := fact.ClientFor(testRepository.Host())
	if err != nil {
		return
	}
	_, err = client.Manifest(testRepository, img.ID.Tag)
	if err == nil {
		t.Fatal("Expected error due to invalid host but got none.")
	}
}

var (
	user string = "user"
	pass string = "pass"
	host string = "host"
	tmpl string = `
    {
        "auths": {
            %q: {"auth": %q}
        }
    }`
	okCreds string = base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
)

func writeCreds(t *testing.T, creds string) (string, func()) {
	file, err := ioutil.TempFile("", "testcreds")
	file.Write([]byte(creds))
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	return file.Name(), func() {
		os.Remove(file.Name())
	}
}

func TestRemoteFactory_CredentialsFromFile(t *testing.T) {
	file, cleanup := writeCreds(t, fmt.Sprintf(tmpl, host, okCreds))
	defer cleanup()

	creds, err := CredentialsFromFile(file)
	if err != nil {
		t.Fatal(err)
	}
	c := creds.credsFor(host)
	if user != c.username {
		t.Fatalf("Expected %q, got %q.", user, c.username)
	}
	if pass != c.password {
		t.Fatalf("Expected %q, got %q.", pass, c.password)
	}
	if len(creds.Hosts()) != 1 || host != creds.Hosts()[0] {
		t.Fatalf("Expected %q, got %q.", host, creds.Hosts()[0])
	}
}

func TestRemoteFactory_CredentialsFromConfigDecodeError(t *testing.T) {
	file, cleanup := writeCreds(t, `{
    "auths": {
        "host": {"auth": "credentials:notencoded"}
    }
}`)
	defer cleanup()
	_, err := CredentialsFromFile(file)
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestRemoteFactory_CredentialsFromConfigHTTPSHosts(t *testing.T) {
	httpsHost := fmt.Sprintf("https://%s/v1/", host)
	file, cleanup := writeCreds(t, fmt.Sprintf(tmpl, httpsHost, okCreds))
	defer cleanup()

	creds, err := CredentialsFromFile(file)
	if err != nil {
		t.Fatal(err)
	}
	c := creds.credsFor(host)
	if user != c.username {
		t.Fatalf("Expected %q, got %q.", user, c.username)
	}
	if pass != c.password {
		t.Fatalf("Expected %q, got %q.", pass, c.password)
	}
	if len(creds.Hosts()) != 1 || host != creds.Hosts()[0] {
		t.Fatalf("Expected %q, got %q.", httpsHost, creds.Hosts()[0])
	}
}

func TestRemoteFactory_ParseHost(t *testing.T) {
	for _, v := range []struct {
		host        string
		imagePrefix string
		error       bool
	}{
		{
			host:        "host",
			imagePrefix: "host",
		},
		{
			host:        "gcr.io",
			imagePrefix: "gcr.io",
		},
		{
			host:        "https://gcr.io",
			imagePrefix: "gcr.io",
		},
		{
			host:        "https://gcr.io/v1",
			imagePrefix: "gcr.io",
		},
		{
			host:        "https://gcr.io/v1/",
			imagePrefix: "gcr.io",
		},
		{
			host:        "gcr.io/v1",
			imagePrefix: "gcr.io",
		},
		{
			host:        "telnet://gcr.io/v1",
			imagePrefix: "gcr.io",
		},
		{
			host:        "",
			imagePrefix: "gcr.io",
			error:       true,
		},
		{
			host:        "https://",
			imagePrefix: "gcr.io",
			error:       true,
		},
		{
			host:        "^#invalid.io/v1/",
			imagePrefix: "gcr.io",
			error:       true,
		},
		{
			host:        "/var/user",
			imagePrefix: "gcr.io",
			error:       true,
		},
	} {

		file, cleanup := writeCreds(t, fmt.Sprintf(tmpl, v.host, okCreds))
		defer cleanup()
		creds, err := CredentialsFromFile(file)
		if (err != nil) != v.error {
			t.Fatalf("For test %q, expected error = %v but got %v", v.host, v.error, err != nil)
		}
		if v.error {
			continue
		}
		if u := creds.credsFor(v.imagePrefix).username; u != user {
			t.Fatalf("For test %q, expected %q but got %v", v.host, user, u)
		}
	}
}

const testTagStr = "tag"
const testImageStr = "index.docker.io/test/Image:" + testTagStr
const constTime = "2017-01-13T16:22:58.009923189Z"

var (
	img, _         = flux.ParseImage(testImageStr, time.Time{})
	testRepository = RepositoryFromImage(img)

	man = schema1.SignedManifest{
		Manifest: schema1.Manifest{
			History: []schema1.History{
				{
					V1Compatibility: `{"created":"` + constTime + `"}`,
				},
			},
		},
	}
)

func TestRepository_ParseImage(t *testing.T) {
	for _, x := range []struct {
		test     string
		expected string
	}{
		{"alpine", "index.docker.io/library/alpine"},
		{"library/alpine", "index.docker.io/library/alpine"},
		{"alpine:mytag", "index.docker.io/library/alpine"},
		{"quay.io/library/alpine", "quay.io/library/alpine"},
		{"quay.io/library/alpine:latest", "quay.io/library/alpine"},
		{"quay.io/library/alpine:mytag", "quay.io/library/alpine"},
	} {
		i, err := ParseRepository(x.test)
		if err != nil {
			t.Fatalf("Failed parsing %q, expected %q", x.test, x.expected)
		}
		if i.String() != x.expected {
			t.Fatalf("%q does not match expected %q", i.String(), x.expected)
		}
	}
}
