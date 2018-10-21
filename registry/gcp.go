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

func GetGCPOauthToken(host string) (credential, error) {
	request, err := http.NewRequest("GET", gcpDefaultTokenURL, nil)
	if err != nil {
		return credential{}, err
	}

	request.Header.Add("Metadata-Flavor", "Google")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return credential{}, err
	}

	if response.StatusCode != http.StatusOK {
		return credential{}, fmt.Errorf("unexpected status from metadata service: %s", response.Status)
	}

	var token gceToken
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&token); err != nil {
		return credential{}, err
	}

	if err := response.Body.Close(); err != nil {
		return credential{}, err
	}

	return credential{
		registry:   host,
		provenance: "",
		username:   "oauth2accesstoken",
		password:   token.AccessToken}, nil
}
