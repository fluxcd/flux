package flux

import (
	"encoding/base64"
	"testing"
)

func TestConfig_ParseSettingsPath(t *testing.T) {
	// We're using SafeInstanceConfig here, but it isn't actually safe
	// because we haven't done InstanceConfig.HideSecrets()
	cfg := SafeInstanceConfig{
		Git: GitConfig{
			Branch: "exampleBranch",
		},
		Registry: RegistryConfig{
			Auths: map[string]Auth{
				"https://index.docker.io/v1/": {
					Auth: "dXNlcjpwYXNzd29yZA==",
				},
			},
		},
	}

	for _, v := range []struct {
		Key    string
		Value  string
		Valid  bool
		Syntax string
	}{
		{"git.branch", "exampleBranch", true, ""},                                               // Get a set parameter and empty syntax
		{"slack.hookURL", "", true, ""},                                                         // Get an unset parameter
		{"does.not.exist", "", false, ""},                                                       // Get a parameter that doesn't exist
		{"registry.auths.'https://index.docker.io/v1/'.auth", "dXNlcjpwYXNzd29yZA==", true, ""}, // Get a map value
		{"git.branch", "exampleBranch", true, "json"},                                           // Test that json syntax works
		{"git.branch", "exampleBranch", true, "yaml"},                                           // Test that yaml syntax works
		{"git.branch", "", false, "invalid"},                                                    // Test that invalid syntax doesn't work
	} {
		resp := cfg.FindSetting(v.Key, v.Syntax)
		if resp.IsValid() != v.Valid {
			t.Fatal(v.Key, "IsValid =", resp.IsValid())
		}
		if resp.IsValid() && resp.String() != v.Value {
			t.Fatalf("%s: Expected %q but got %q", v.Key, v.Value, resp.String())
		}
	}
}

func TestConfig_WriteSetting(t *testing.T) {
	// We're using SafeInstanceConfig here, but it isn't actually safe
	// because we haven't done InstanceConfig.HideSecrets()
	cfg := InstanceConfig{
		Git: GitConfig{
			Branch: "exampleBranch",
		},
		Registry: RegistryConfig{
			Auths: map[string]Auth{
				"aaaa": {
					Auth: base64.StdEncoding.EncodeToString([]byte(`user:password`)),
				},
				"https://index.docker.io/v1/": {
					Auth: base64.StdEncoding.EncodeToString([]byte(`user:password`)),
				},
			},
		},
	}

	for _, v := range []struct {
		Key    string
		Value  string
		Result string
		IsErr  bool
		Syntax string
	}{
		{"git.branch", "newBranch", "newBranch", false, ""}, // Set a previously set value
		{"git.URL", "newUrl", "newUrl", false, ""},          // Set on an empty value
		{"git.abc", "", "", true, ""},                       // Set a param that doesn't exist
		{"registry.auths.'https://index.docker.io/v1/'.auth", base64.StdEncoding.EncodeToString([]byte(`new:pass`)), "new:******", false, ""}, // Set a map value
	} {
		err := cfg.WriteSetting(SingleConfigParams{v.Key, ConfigSyntax(v.Syntax)}, v.Value)
		if (err != nil) != v.IsErr {
			t.Fatalf("%q: Expected error = %v, but got %v.", v.Key, v.IsErr, err != nil)
		}
		if !v.IsErr {
			s := cfg.HideSecrets().FindSetting(v.Key, "").String()
			if s != v.Result {
				t.Fatalf("Setting has not changed for key %q. Expected %q but still %q\n", v.Key, v.Value, s)
			}
		}
	}

}
