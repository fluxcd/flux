package flux

import (
	"encoding/json"
	"testing"
)

func TestConfig_Patch(t *testing.T) {

	uic := UnsafeInstanceConfig{
		Git: GitConfig{
			Key: "existingkey",
		},
		Registry: RegistryConfig{
			Auths: map[string]Auth{
				"https://index.docker.io/v1/": {
					Auth: "existingauth",
				},
			},
		},
	}

	patchBytes := []byte(`{
		"git": {
			"key": "newkey"
		},
		"registry": { 
			"auths": { 
				"https://index.docker.io/v1/": null,
				"quay.io": {
					"Auth": "some auth config"
				}
			}
		}
	}`)

	var cf ConfigPatch
	if err := json.Unmarshal(patchBytes, &cf); err != nil {
		t.Fatal(err)
	}

	puic, err := uic.Patch(cf)
	if err != nil {
		t.Fatal(err)
	}

	if puic.Git.Key != "newkey" {
		t.Fatalf("git key not patched: %v", puic.Git.Key)
	}

	if len(puic.Registry.Auths) != 1 || puic.Registry.Auths["quay.io"].Auth != "some auth config" {
		t.Fatal("auth config not patched")
	}
}
