package registry

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
)

// Note: This actually goes off to docker hub to find the Image.
// It will fail if there is not internet connection
func TestRemoteFactory_CreateForDockerHub(t *testing.T) {
	// No credentials required for public Image
	fact := NewRemoteClientFactory(Credentials{}, log.NewNopLogger(), RateLimiterConfig{
		RPS:   200,
		Burst: 1,
	})
	img, err := flux.ParseImage("alpine:latest", time.Time{})
	testRepository = RepositoryFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	client, cancel, err := fact.ClientFor(testRepository.Host())
	if err != nil {
		t.Fatal(err)
	}
	r := newRemote(client, cancel)
	res, err := r.Manifest(testRepository, img.ID.Tag)
	if err != nil {
		t.Fatal(err)
	}
	expected := "index.docker.io/library/alpine:latest"
	if res.ID.FullID() != expected {
		t.Fatal("Expected %q. Got %q", expected, res.ID.FullID())
	}
}

func TestRemoteFactory_RawClient(t *testing.T) {
	// No credentials required for public Image
	fact := NewRemoteClientFactory(Credentials{}, log.NewNopLogger(), RateLimiterConfig{
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
	client, cancel, err := fact.ClientFor(testRepository.Host())
	if err != nil {
		t.Fatal(err)
	}

	tags, err = client.Tags(testRepository.NamespaceImage())
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if len(tags) == 0 {
		t.Fatal("Should have some tags")
	}

	client, cancel, err = fact.ClientFor(testRepository.Host())
	if err != nil {
		t.Fatal(err)
	}
	history, err := client.Manifest(testRepository.NamespaceImage(), tags[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(history) == 0 {
		t.Fatal("Should have some history")
	}
	cancel()
}

func TestRemoteFactory_InvalidHost(t *testing.T) {
	fact := NewRemoteClientFactory(Credentials{}, log.NewNopLogger(), RateLimiterConfig{})
	img, err := flux.ParseImage("invalid.host/library/alpine:latest", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	testRepository = RepositoryFromImage(img)
	client, cancel, err := fact.ClientFor(testRepository.Host())
	if err != nil {
		return
	}
	r := newRemote(client, cancel)
	_, err = r.Manifest(testRepository, img.ID.Tag)
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
