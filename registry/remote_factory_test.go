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
	fact := NewRemoteClientFactory(Credentials{}, log.NewNopLogger(), nil, time.Second)
	img, err := flux.ParseImage("alpine:latest", time.Time{})
	testRepository = RepositoryFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	r, err := fact.CreateFor(testRepository.Host())
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Manifest(testRepository, img.ID.Tag)
	if err != nil {
		t.Fatal(err)
	}
	expected := "index.docker.io/library/alpine:latest"
	if res.ID.FullID() != expected {
		t.Fatal("Expected %q. Got %q", expected, res.ID.FullID())
	}
}

func TestRemoteFactory_InvalidHost(t *testing.T) {
	fact := NewRemoteClientFactory(Credentials{}, log.NewNopLogger(), nil, time.Second)
	img, err := flux.ParseImage("invalid.host/library/alpine:latest", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	testRepository = RepositoryFromImage(img)
	r, err := fact.CreateFor(testRepository.Host())
	if err != nil {
		t.Fatal(err)
	}
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
	if len(creds.Hosts()) != 1 || httpsHost != creds.Hosts()[0] {
		t.Fatalf("Expected %q, got %q.", httpsHost, creds.Hosts()[0])
	}
}
