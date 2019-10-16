package registry

import (
	"encoding/json"
	"io/ioutil"
	"strings"
)

const (
	// Mount volume from hostpath.
	azureCloudConfigJsonFile = "/etc/kubernetes/azure.json"
)

type azureCloudConfig struct {
	AADClientId     string `json:"aadClientId"`
	AADClientSecret string `json:"aadClientSecret"`
}

// Fetch Azure Active Directory clientid/secret pair from azure.json, usable for container registry authentication.
//
// Note: azure.json is populated by AKS/AKS-Engine script kubernetesconfigs.sh. The file is then passed to kubelet via
// --azure-container-registry-config=/etc/kubernetes/azure.json, parsed by kubernetes/kubernetes' azure_credentials.go
// https://github.com/kubernetes/kubernetes/issues/58034 seeks to deprecate this kubelet command-line argument, possibly
// replacing it with managed identity for the Node VMs. See https://github.com/Azure/acr/blob/master/docs/AAD-OAuth.md
func getAzureCloudConfigAADToken(host string) (creds, error) {
	jsonFile, err := ioutil.ReadFile(azureCloudConfigJsonFile)
	if err != nil {
		return creds{}, err
	}

	var token azureCloudConfig

	err = json.Unmarshal(jsonFile, &token)
	if err != nil {
		return creds{}, err
	}

	return creds{
		registry:   host,
		provenance: "azure.json",
		username:   token.AADClientId,
		password:   token.AADClientSecret}, nil
}

// List from https://github.com/kubernetes/kubernetes/blob/master/pkg/credentialprovider/azure/azure_credentials.go
func hostIsAzureContainerRegistry(host string) bool {
	for _, v := range []string{".azurecr.io", ".azurecr.cn", ".azurecr.de", ".azurecr.us"} {
		if strings.HasSuffix(host, v) {
			return true
		}
	}
	return false
}
