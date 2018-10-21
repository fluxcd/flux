package registry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	credentialJSONResponse = `{
"auths": {
	"https://12345.dkr.ecr-use-east-1.amazonaws.com": {
		"auth": "QVdTOmVjcnBhc3N3b3JkCg=="
	},
	"https://602401143452.dkr.ecr.us-east-1.amazonaws.com": {
		"auth": "QVdTOnh4eHh4Cg=="
	}
}}`
	awsUser                    = "AWS"
	credentialHost1            = "12345.dkr.ecr-use-east-1.amazonaws.com"
	credentialDecodedPassword1 = "ecrpassword"
	credentialHost2            = "602401143452.dkr.ecr.us-east-1.amazonaws.com"
	credentialDecodedPassword2 = "xxxxx"
)

func TestGetECRRegistryCredential(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(credentialJSONResponse))
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	tt := []struct {
		pass             bool
		expectedUser     string
		expectedPassword string
		registryHost     string
	}{
		{true, awsUser, credentialDecodedPassword1, credentialHost1},
		{true, awsUser, credentialDecodedPassword2, credentialHost2},
		{false, awsUser, "qweqwewq", "unknownprofile.dkr.ecr.us-east-1.amazonaws.com"},
	}

	for _, tc := range tt {
		if tc.pass {
			t.Run("known registry host must pass", func(t *testing.T) {
				creds, err := GetECRRegistryCredential(tc.registryHost, ts.URL)
				if err != nil {
					t.Fatal(err)
				}
				if creds.username != tc.expectedUser {
					t.Fatalf("incorrect decoded username: got %s, want %s!", creds.username, tc.expectedUser)
				}
				if creds.password != tc.expectedPassword {
					t.Fatalf("incorrect decoded password. got %s, want %s!", creds.password, tc.expectedPassword)
				}
				fmt.Println("--- INFO: got ", creds)
			})
		} else {
			t.Run("unknown registry host must fail", func(t *testing.T) {
				_, err := GetECRRegistryCredential(tc.registryHost, ts.URL)
				if err == nil {
					t.Fatalf("expected to fail test when fetching a non existent host %s but it went ok!", tc.registryHost)
				}
			})
		}
	}
}
