package registry

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
			host:        "192.168.99.100:5000",
			imagePrefix: "192.168.99.100:5000",
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
		t.Run(v.host, func(t *testing.T) {
			stringCreds := fmt.Sprintf(tmpl, v.host, okCreds)
			creds, err := ParseCredentials("test", []byte(stringCreds))
			if (err != nil) != v.error {
				t.Fatalf("For test %q, expected error = %v but got %v", v.host, v.error, err != nil)
			}
			if v.error {
				return
			}
			actualUser := creds.credsFor(v.imagePrefix).username
			assert.Equal(t, user, actualUser, "For test %q, expected %q but got %v", v.host, user, actualUser)
			actualPass := creds.credsFor(v.imagePrefix).password
			assert.Equal(t, pass, actualPass, "For test %q, expected %q but got %v", v.host, user, actualPass)
		})
	}
}

func TestParseCreds_k8s(t *testing.T) {
	k8sCreds := []byte(`{"localhost:5000":{"username":"testuser","password":"testpassword","email":"foo@bar.com","auth":"dGVzdHVzZXI6dGVzdHBhc3N3b3Jk"}}`)
	c, err := ParseCredentials("test", k8sCreds)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c.Hosts()), "Invalid number of hosts")
	host := c.Hosts()[0]
	assert.Equal(t, "localhost:5000", host, "Host is incorrect")
	assert.Equal(t, "testuser", c.credsFor(host).username, "User is incorrect")
	assert.Equal(t, "testpassword", c.credsFor(host).password, "Password is incorrect")
}

func TestStringShouldNotLeakPasswords(t *testing.T) {
	k8sCreds := []byte(`{"localhost:5000":{"username":"testuser","password":"testpassword","email":"foo@bar.com","auth":"dGVzdHVzZXI6dGVzdHBhc3N3b3Jk"}}`)
	c, err := ParseCredentials("test", k8sCreds)
	assert.NoError(t, err)
	assert.Equal(t, "{map[localhost:5000:<registry creds for testuser@localhost:5000, from test>]}", fmt.Sprintf("%v", c)) // In comparison standard String() method typically yields: "{map[localhost:5000:{testuser testpassword localhost:5000 test}]}".
	assert.Equal(t, "testpassword", c.credsFor("localhost:5000").password, "Password is incorrect")                        // Actual password is left untouched.
}
