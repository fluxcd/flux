package release

import (
	"strings"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
	yaml "gopkg.in/yaml.v2"
)

// collectValues ... assembles overriding parameters and outputs a
// serialised map suitable for passing to helm client API.
// Parameters with names containing "." will result in nested maps.
// Maps from all parameters are merged together, with a possible overwriting
// if a parameter is provided twice with different values.
func collectValues(params []ifv1.HelmChartParam) ([]byte, error) {
	base := map[string]interface{}{}
	if params == nil || len(params) == 0 {
		return yaml.Marshal(base)
	}

	for _, p := range params {
		k, v := cleanup(p.Name, p.Value)
		if k == "" {
			continue
		}

		pMap := mappifyValueOverride(k, v)
		base = mergeValues(base, pMap)
	}

	return yaml.Marshal(base)
}

func cleanup(k, v string) (string, string) {
	k = strings.TrimSpace(k)
	k = strings.Trim(k, "\n")

	v = strings.TrimSpace(v)
	v = strings.Trim(v, "\n")

	return k, v
}

func reverse(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// mappifyValueOverride ... takes a parameter and its value, and creates
// a corresponding map suitable for passing to helm client API
// to override default values
func mappifyValueOverride(k, v string) map[string]interface{} {
	nests := reverse(strings.Split(k, "."))

	inner := map[string]interface{}{}
	outer := map[string]interface{}{}
	for i, n := range nests {
		switch i {
		case 0:
			inner[n] = v
		default:
			outer = map[string]interface{}{
				nests[i]: inner,
			}
			inner = outer
		}

	}
	return inner
}

// mergeValues ... merges two, possibly nested, maps
// (copied from kubernetes/helm/cmd/helm/install.go)
func mergeValues(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = nextMap
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}
