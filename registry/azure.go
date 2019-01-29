package registry

import (
	"encoding/json"
	"os"
	yaml "gopkg.in/yaml.v2"
)

const (
    // Mount volume from hostpath.
    azureCloudConfigJsonFile = "/etc/kubernetes/azure.json"

    // List from https://github.com/kubernetes/kubernetes/blob/master/pkg/credentialprovider/azure/azure_credentials.go
    azureContainerRegistryHosts = []string{".azurecr.io", ".azurecr.cn", ".azurecr.de", ".azurecr.us"};
)

type azureCloudConfig struct {
    aadClientId string `json:"aadClientId"`
    aadClientSecret string `json:"aadClientSecret"`
}

// Fetch Azure Active Directory clientid/secret pair from azure.json, usable for container registry authentication.
//
// Note: azure.json is populated by AKS/AKS-Engine script kubernetesconfigs.sh. The file is then passed to kubelet via
// --azure-container-registry-config=/etc/kubernetes/azure.json, parsed by kubernetes/kubernetes' azure_credentials.go
// https://github.com/kubernetes/kubernetes/issues/58034 seeks to deprecate this kubelet command-line argument, possibly
// replacing it with managed identity for the Node VMs. See https://github.com/Azure/acr/blob/master/docs/AAD-OAuth.md
func GetAzureCloudConfigAADToken(host string) (creds, error) {
    jsonFile, err := os.Open(azureCloudConfigJsonFile)
    if err != nil {
        return creds{}, err
    }
    defer jsonFile.Close()

    var token azureCloudConfig
    decoder := json.NewDecoder(jsonFile)
    if err := decoder.Decode(&token); err != nil {
        return creds{}, err
    }

    return creds{
        registry:   host,
        provenance: "azure.json",
        username:   token.aadClientId,
        password:   token.aadClientSecret}, nil
}

func HostIsAzureContainerRegistry(host string) (bool) {
    for _, v := range azureContainerRegistryHosts {
        if strings.HasSuffix(host, v) {
            return true
        }
    }
    return false
}
