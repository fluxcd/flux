package ecr

import (
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
)

// DockerCredential is used to decode the output of ecr.GetAuthorizationToken
// to a docker config JSON credential string.
//  {
//   "auths": {
//     "{{ registry endpoint }}": {
//        "auth": "{{ authentication token }}"
//     }
//    }
//  }
type DockerCredential struct {
	Auths map[string]Auth `json:"auths"`
}

// Auth contains the ECR base64 encoded username:password.
type Auth struct {
	Auth string `json:"auth"`
}

// GetAmazonECRToken fetches ECR Docker credentials of the given AWS Accounts or ECR Registry IDs.
func GetAmazonECRToken(svc ecriface.ECRAPI, registryIDs []string) (*DockerCredential, error) {
	ecrToken, err := svc.GetAuthorizationToken(
		&ecr.GetAuthorizationTokenInput{
			RegistryIds: aws.StringSlice(registryIDs),
		},
	)
	if err != nil {
		return nil, err
	}
	auths := make(map[string]Auth)
	for _, v := range ecrToken.AuthorizationData {
		// Remove the https prefix
		host := strings.TrimPrefix(*v.ProxyEndpoint, "https://")
		auths[host] = Auth{*v.AuthorizationToken}
	}
	return &DockerCredential{auths}, nil
}

func (cred *DockerCredential) String() string {
	bs, _ := json.Marshal(cred)
	return string(bs)
}
