package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/weaveworks/flux/sidecar/aws/ecr"
)

// GetECRRegistryCredential is used by the Flux daemon to communicate to the AWS sidecar url
// to GET the appropriate ECR repository's docker username and password. If the input
// host was not found from the sidecar's JSON response, an empty credential will be returned.
func GetECRRegistryCredential(host, sidecarURL string) (credential, error) {
	res, err := http.Get(sidecarURL)
	if err != nil {
		return credential{}, nil
	}
	defer res.Body.Close()
	cred := &ecr.DockerCredential{}
	if err := json.NewDecoder(res.Body).Decode(&cred); err != nil {
		return credential{}, nil
	}
	for auth, entry := range cred.Auths {
		hostRegistryID := strings.Split(host, ".")[0]
		authHost := strings.TrimPrefix(auth, "https://")
		authRegistryID := strings.Split(authHost, ".")[0]
		if hostRegistryID == authRegistryID {
			decodedAuth, err := base64.StdEncoding.DecodeString(entry.Auth)
			if err != nil {
				return credential{}, err
			}
			authParts := strings.SplitN(string(decodedAuth), ":", 2)
			return credential{
				registry:   host,
				provenance: ecr.SidecarAWSURL,
				username:   authParts[0],
				password:   strings.TrimSpace(authParts[1]),
			}, nil
		}
	}
	return credential{},
		fmt.Errorf("%s: unable to find auth for host %s", ecr.SidecarAWSURL, host)
}
