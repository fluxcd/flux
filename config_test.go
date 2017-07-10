package flux

import (
	"encoding/json"
	"testing"
)

func TestConfig_Patch(t *testing.T) {

	uic := InstanceConfig{
		NotifierConfig{
			HookURL: "existingurl",
		},
	}

	patchBytes := []byte(`{
		"slack": {
			"hookURL": "newurl"
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

	if puic.Slack.HookURL != "newurl" {
		t.Fatalf("slack hookURL not patched: %v", puic.Slack.HookURL)
	}
}
