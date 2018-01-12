package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	gcpDefaultTokenURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
)

type gceToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func GetGCPOauthToken(host string) (creds, error) {
	request, err := http.NewRequest("GET", gcpDefaultTokenURL, nil)
	if err != nil {
		return creds{}, err
	}

	request.Header.Add("Metadata-Flavor", "Google")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return creds{}, err
	}

	if response.StatusCode != http.StatusOK {
		return creds{}, fmt.Errorf("unexpected status from metadata service: %s", response.Status)
	}

	var token gceToken
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&token); err != nil {
		return creds{}, err
	}

	if err := response.Body.Close(); err != nil {
		return creds{}, err
	}

	return creds{
		registry:   host,
		provenance: "",
		username:   "oauth2accesstoken",
		password:   token.AccessToken}, nil
}
