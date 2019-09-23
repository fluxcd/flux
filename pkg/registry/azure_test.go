package registry

import (
	"testing"
)

func Test_HostIsAzureContainerRegistry(t *testing.T) {
	for _, v := range []struct {
		host  string
		isACR bool
	}{
		{
			host:  "azurecr.io",
			isACR: false,
		},
		{
			host:  "",
			isACR: false,
		},
		{
			host:  "gcr.io",
			isACR: false,
		},
		{
			host:  "notazurecr.io",
			isACR: false,
		},
		{
			host:  "example.azurecr.io.not",
			isACR: false,
		},
		// Public cloud
		{
			host:  "example.azurecr.io",
			isACR: true,
		},
		// Sovereign clouds
		{
			host:  "example.azurecr.cn",
			isACR: true,
		},
		{
			host:  "example.azurecr.de",
			isACR: true,
		},
		{
			host:  "example.azurecr.us",
			isACR: true,
		},
	} {
		result := hostIsAzureContainerRegistry(v.host)
		if result != v.isACR {
			t.Fatalf("For test %q, expected isACR = %v but got %v", v.host, v.isACR, result)
		}
	}
}
