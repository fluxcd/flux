package registry

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

var (
	user string = "user"
	pass string = "pass"
	tmpl string = `
    {
        "auths": {
            %q: {"auth": %q}
        }
    }`
	okCreds string = base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
)

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
			host:        "localhost:5000/v2/",
			imagePrefix: "localhost:5000",
		},
		{
			host:        "https://192.168.99.100:5000/v2",
			imagePrefix: "192.168.99.100:5000",
		},
		{
			host:        "https://my.domain.name:5000/v2",
			imagePrefix: "my.domain.name:5000",
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
			host:  "",
			error: true,
		},
		{
			host:  "https://",
			error: true,
		},
		{
			host:  "^#invalid.io/v1/",
			error: true,
		},
		{
			host:  "/var/user",
			error: true,
		},
	} {
		stringCreds := fmt.Sprintf(tmpl, v.host, okCreds)
		creds, err := ParseCredentials([]byte(stringCreds))
		time.Sleep(100 * time.Millisecond)
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

func TestParseCreds_k8s(t *testing.T) {
	k8sCreds := []byte(`{"localhost:5000":{"username":"testuser","password":"testpassword","email":"foo@bar.com","auth":"dGVzdHVzZXI6dGVzdHBhc3N3b3Jk"}}`)
	c, err := ParseCredentials(k8sCreds)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Hosts()) != 1 {
		t.Fatal("Invalid number of hosts", len(c.Hosts()))
	} else if c.Hosts()[0] != "localhost:5000" {
		t.Fatal("Host is incorrect: ", c.Hosts()[0])
	} else if c.credsFor("localhost:5000").username != "testuser" {
		t.Fatal("Invalid user", c.credsFor("localhost:5000").username)
	} else if c.credsFor("localhost:5000").password != "testpassword" {
		t.Fatal("Invalid user", c.credsFor("localhost:5000").password)
	}
}
