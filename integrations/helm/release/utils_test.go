package release

import (
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
)

var noopLogger = log.NewNopLogger()

// mappifyValueOverride tests
type mappifyTestCase struct {
	desc     string
	param    string
	value    string
	expected map[string]interface{}
}

var mappifyExpectedSimple = map[string]interface{}{
	"image": "bitnami/kibana:master-1",
}
var mappifyExpectedNested2 = map[string]interface{}{
	"image": map[string]interface{}{
		"repository": "bitnami/kibana:master-1",
	},
}
var mappifyExpectedNested3 = map[string]interface{}{
	"image": map[string]interface{}{
		"repository": map[string]interface{}{
			"name": "bitnami/kibana:master-1",
		},
	},
}

func TestMappifyValueOverride(t *testing.T) {
	for _, c := range []mappifyTestCase{
		{"simple parameter", "image", "bitnami/kibana:master-1", mappifyExpectedSimple},
		{"nested parameter: 2 levels", "image.repository", "bitnami/kibana:master-1", mappifyExpectedNested2},
		{"nested parameter: 3 levels", "image.repository.name", "bitnami/kibana:master-1", mappifyExpectedNested3},
	} {
		testMappify(t, c)
	}
}

func testMappify(t *testing.T, c mappifyTestCase) {
	result := mappifyValueOverride(c.param, c.value)

	if !reflect.DeepEqual(c.expected, result) {
		t.Fatalf("\n%s [%s, %s]\nDid not get the expected result:\n\n{%v}\n\nInstead got:\n\n{%v}", c.desc, c.param, c.value, c.expected, result)
	}
}

// ---------------------------------------------------------------------------
// collectValues tests
type collectTestCase struct {
	desc     string
	params   []ifv1.HelmChartParam
	expected map[string]interface{}
}

var collectSimpleSimpleParams = []ifv1.HelmChartParam{
	{Name: "image", Value: "bitnami/kibana"},
	{Name: "tag", Value: "alpha"},
	{Name: "memory", Value: "1Gi"},
}
var collectExpectedSimpleSimple = map[string]interface{}{
	"image":  "bitnami/kibana",
	"tag":    "alpha",
	"memory": "1Gi",
}

var collectSimpleListParams = []ifv1.HelmChartParam{
	{Name: "image", Value: "bitnami/kibana"},
	{Name: "tag", Value: "alpha"},
	{Name: "tolerances", Value: `[{"key":"aaa","value":"val"},{"key":"bbb","value":"val2"}]`},
}
var collectExpectedSimpleList = map[string]interface{}{
	"image":      "bitnami/kibana",
	"tag":        "alpha",
	"tolerances": []interface{}{map[string]interface{}{"key": "aaa", "value": "val"}, map[string]interface{}{"key": "bbb", "value": "val2"}}}

var collectNestedSimpleParams = []ifv1.HelmChartParam{
	{Name: "image.repository.name", Value: "bitnami/kibana"},
	{Name: "resources.memory", Value: "1Gi"},
}
var collectExpectedNestedSimple = map[string]interface{}{
	"image": map[string]interface{}{
		"repository": map[string]interface{}{
			"name": "bitnami/kibana",
		},
	},
	"resources": map[string]interface{}{
		"memory": "1Gi",
	},
}

var collectNestedListParams = []ifv1.HelmChartParam{
	{Name: "image.repository.name", Value: `[{"key":"aaa","value":"val1"},{"key":"bbb","value":"val2"}]`},
	{Name: "resources.memory", Value: `[{"key":"ccc","value":"val3"},{"key":"ddd","value":"val4"}]`},
}
var collectExpectedNestedList = map[string]interface{}{
	"image":     map[string]interface{}{"repository": map[string]interface{}{"name": []interface{}{map[string]interface{}{"key": "aaa", "value": "val1"}, map[string]interface{}{"key": "bbb", "value": "val2"}}}},
	"resources": map[string]interface{}{"memory": []interface{}{map[string]interface{}{"key": "ccc", "value": "val3"}, map[string]interface{}{"key": "ddd", "value": "val4"}}}}

var collectNestedMixtureParams = []ifv1.HelmChartParam{
	{Name: "image.repository.name", Value: "bitnami/kibana"},
	{Name: "resources.memory", Value: `[{"key":"aaa","value":"val"},{"key":"bbb","value":"val2"}]`},
}
var collectExpectedNestedMixture = map[string]interface{}{
	"image": map[string]interface{}{
		"repository": map[string]interface{}{
			"name": "bitnami/kibana",
		},
	},
	"resources": map[string]interface{}{"memory": []interface{}{map[string]interface{}{"key": "aaa", "value": "val"}, map[string]interface{}{"key": "bbb", "value": "val2"}}}}

func TestCollectValues(t *testing.T) {
	for _, c := range []collectTestCase{
		{"nil params", nil, map[string]interface{}{}},
		{"nil params", []ifv1.HelmChartParam{}, map[string]interface{}{}},
		{"simple parameters, simple value", collectSimpleSimpleParams, collectExpectedSimpleSimple},
		{"simple parameters: list value", collectSimpleListParams, collectExpectedSimpleList},
		{"nested parameters: simple values", collectNestedSimpleParams, collectExpectedNestedSimple},
		{"nested parameters: list values", collectNestedListParams, collectExpectedNestedList},
		{"nested parameters: simple and list values", collectNestedMixtureParams, collectExpectedNestedMixture},
	} {
		testCollect(t, c)
	}
}

func testCollect(t *testing.T, c collectTestCase) {
	result, err := collectValues(noopLogger, c.params)
	if err != nil {
		t.Fatalf("\n%s [%v]\n%v", c.desc, c.params, err)
	}
	if !reflect.DeepEqual(c.expected, result) {
		t.Fatalf("\n%s [%v]\nDid not get the expected result:\n\n%v\n\nInstead got:\n\n%v)", c.desc, c.params, c.expected, result)
	}
}

// ---------------------------------------------------------------------------
// unwrap tests
type unwrapTestCase struct {
	desc     string
	value    string
	expected []interface{}
}

func TestUnwrap(t *testing.T) {
	for _, c := range []unwrapTestCase{
		{"list with simple values", `["AAA","BBB","CCC"]`, []interface{}{"AAA", "BBB", "CCC"}},
		{"list with maps", `[{"key":"DDD","value":"ddd"},{"key":"EEE","value":"eee"}]`, []interface{}{map[string]interface{}{"key": "DDD", "value": "ddd"}, map[string]interface{}{"key": "EEE", "value": "eee"}}},
	} {
		testUnwrap(t, c)
	}
}

func testUnwrap(t *testing.T, c unwrapTestCase) {
	result, err := unwrap(c.value)
	if err != nil {
		t.Fatalf("\n%s [%s]\n%v", c.desc, c.value, err)
	}
	if !reflect.DeepEqual(c.expected, result) {
		t.Fatalf("\n%s [%s]\nDid not get the expected result:\n\n{%v}\n\nInstead got:\n\n{%v}", c.desc, c.value, c.expected, result)
	}
}

// ---------------------------------------------------------------------------
// mergeOverrides tests
type mergeTestCase struct {
	desc     string
	dest     map[string]interface{}
	src      map[string]interface{}
	expected map[string]interface{}
}

func TestMergeOverrides(t *testing.T) {
	for _, c := range []mergeTestCase{
		{"dest = empty and src = map",
			map[string]interface{}{},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
		},
		{"dest = empty and src != map",
			map[string]interface{}{},
			map[string]interface{}{"AAA": "333"},
			map[string]interface{}{"AAA": "333"},
		},
		{"dest = empty and src = list",
			map[string]interface{}{},
			map[string]interface{}{"AAA": []interface{}{"111", "222"}},
			map[string]interface{}{"AAA": []interface{}{"111", "222"}},
		},
		{"dest = map and src = primitive",
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"AAA": "222"},
			map[string]interface{}{"AAA": "222"},
		},
		{"dest = map and src = list",
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"AAA": []interface{}{"111", "222"}},
			map[string]interface{}{"AAA": []interface{}{"111", "222"}}},
		{"dest != map and src = map",
			map[string]interface{}{"AAA": "222"},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
		},
		{"both dest and src are maps - different keys",
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"BBB": map[string]interface{}{"bbb": "222"}},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}, "BBB": map[string]interface{}{"bbb": "222"}},
		},
		{"both dest and src are maps - overlapping keys a)",
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"AAA": map[string]interface{}{"bbb": "222"}, "BBB": map[string]interface{}{"bbb": "222"}},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111", "bbb": "222"}, "BBB": map[string]interface{}{"bbb": "222"}},
		},
		{"both dest and src are maps - overlapping keys b)",
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}, "CCC": "333"},
			map[string]interface{}{"AAA": map[string]interface{}{"bbb": "222"}, "BBB": map[string]interface{}{"bbb": "222"}},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111", "bbb": "222"}, "BBB": map[string]interface{}{"bbb": "222"}, "CCC": "333"},
		},
		{"both dest and src are maps - overlapping keys c)",
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"AAA": map[string]interface{}{"bbb": "222", "ccc": "333"}, "BBB": map[string]interface{}{"bbb": "222"}},
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111", "bbb": "222", "ccc": "333"}, "BBB": map[string]interface{}{"bbb": "222"}},
		},
		{"both dest and src are maps - overlapping keys d)",
			map[string]interface{}{"AAA": []interface{}{"aaa", "bbb"}},
			map[string]interface{}{"AAA": map[string]interface{}{"bbb": "222", "ccc": "333"}, "BBB": map[string]interface{}{"bbb": "222"}},
			map[string]interface{}{"AAA": map[string]interface{}{"bbb": "222", "ccc": "333"}, "BBB": map[string]interface{}{"bbb": "222"}},
		},
		{"both dest and src are maps - overlapping keys e)",
			map[string]interface{}{"AAA": map[string]interface{}{"aaa": "111"}},
			map[string]interface{}{"AAA": []interface{}{"aaa", "bbb"}, "BBB": map[string]interface{}{"bbb": "222"}},
			map[string]interface{}{"AAA": []interface{}{"aaa", "bbb"}, "BBB": map[string]interface{}{"bbb": "222"}},
		},
	} {
		testMergeOverrides(t, c)
	}
}

func testMergeOverrides(t *testing.T, c mergeTestCase) {
	result := mergeOverrides(c.dest, c.src)

	if !reflect.DeepEqual(c.expected, result) {
		t.Fatalf("\n%s \n[%v\n%v\n]\nDid not get the expected result:\n\n{%#v}\n\nInstead got:\n\n{%#v}", c.desc, c.dest, c.src, c.expected, result)
	}
}
