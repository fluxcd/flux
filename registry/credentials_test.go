package registry

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

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
