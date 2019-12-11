package resource

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
	"github.com/fluxcd/flux/pkg/gpg/gpgtest"
	"github.com/fluxcd/flux/pkg/resource"
)

// for convenience
func base(source, kind, namespace, name string) baseObject {
	b := baseObject{source: source, Kind: kind}
	b.Meta.Namespace = namespace
	b.Meta.Name = name
	return b
}

func TestParseEmpty(t *testing.T) {
	doc := ``

	objs, err := ParseMultidoc([]byte(doc), "test")
	if err != nil {
		t.Error(err)
	}
	if len(objs) != 0 {
		t.Errorf("expected empty set; got %#v", objs)
	}
}

func TestParseSome(t *testing.T) {
	docs := `---
kind: Deployment
metadata:
  name: b-deployment
  namespace: b-namespace
---
kind: Deployment
metadata:
  name: a-deployment
`
	objs, err := ParseMultidoc([]byte(docs), "test")
	if err != nil {
		t.Error(err)
	}

	objA := base("test", "Deployment", "", "a-deployment")
	objB := base("test", "Deployment", "b-namespace", "b-deployment")
	expected := map[string]resource.Resource{
		objA.ResourceID().String(): &Deployment{baseObject: objA},
		objB.ResourceID().String(): &Deployment{baseObject: objB},
	}

	for id, obj := range expected {
		// Remove the bytes, so we can compare
		if !reflect.DeepEqual(obj, debyte(objs[id])) {
			t.Errorf("At %+v expected:\n%#v\ngot:\n%#v", id, obj, objs[id])
		}
	}
}

func TestParseSomeWithComment(t *testing.T) {
	docs := `# some random comment
---
kind: Deployment
metadata:
  name: b-deployment
  namespace: b-namespace
---
kind: Deployment
metadata:
  name: a-deployment
`
	objs, err := ParseMultidoc([]byte(docs), "test")
	if err != nil {
		t.Error(err)
	}

	objA := base("test", "Deployment", "", "a-deployment")
	objB := base("test", "Deployment", "b-namespace", "b-deployment")
	expected := map[string]resource.Resource{
		objA.ResourceID().String(): &Deployment{baseObject: objA},
		objB.ResourceID().String(): &Deployment{baseObject: objB},
	}
	expectedL := len(expected)

	if len(objs) != expectedL {
		t.Errorf("expected %d objects from yaml source\n%s\n, got result: %d", expectedL, docs, len(objs))
	}

	for id, obj := range expected {
		// Remove the bytes, so we can compare
		if !reflect.DeepEqual(obj, debyte(objs[id])) {
			t.Errorf("At %+v expected:\n%#v\ngot:\n%#v", id, obj, objs[id])
		}
	}
}

func TestParseSomeLong(t *testing.T) {
	doc := `---
kind: ConfigMap
metadata:
  name: bigmap
data:
  bigdata: |
`
	buffer := bytes.NewBufferString(doc)
	line := "    The quick brown fox jumps over the lazy dog.\n"
	for buffer.Len()+len(line) < 1024*1024 {
		buffer.WriteString(line)
	}

	_, err := ParseMultidoc(buffer.Bytes(), "test")
	if err != nil {
		t.Error(err)
	}
}

func TestParseBoundaryMarkers(t *testing.T) {
	doc := `---
kind: ConfigMap
metadata:
  name: bigmap
---
...
---
...
---
...
---
...
`
	buffer := bytes.NewBufferString(doc)

	resources, err := ParseMultidoc(buffer.Bytes(), "test")
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
}

func TestParseError(t *testing.T) {
	doc := `---
kind: ConfigMap
metadata:
	name: bigmap # contains a tab at the beginning
`
	buffer := bytes.NewBufferString(doc)

	_, err := ParseMultidoc(buffer.Bytes(), "test")
	assert.Error(t, err)
}

func TestParseCronJob(t *testing.T) {
	doc := `---
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  namespace: default
  name: weekly-curl-homepage
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: weekly-curl-homepage
            image: centos:7 # Has curl installed by default
`
	objs, err := ParseMultidoc([]byte(doc), "test")
	assert.NoError(t, err)

	obj, ok := objs["default:cronjob/weekly-curl-homepage"]
	assert.True(t, ok)
	cj, ok := obj.(*CronJob)
	assert.True(t, ok)

	containers := cj.Spec.JobTemplate.Spec.Template.Spec.Containers
	if assert.Len(t, containers, 1) {
		assert.Equal(t, "centos:7", containers[0].Image)
		assert.Equal(t, "weekly-curl-homepage", containers[0].Name)
	}
}

func TestUnmarshalList(t *testing.T) {
	doc := `---
kind: List
metadata:
  name: list
items:
- kind: Deployment
  metadata:
    name: foo
    namespace: ns
- kind: Service
  metadata:
    name: bar
    namespace: ns
`
	res, err := unmarshalObject("", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	list, ok := res.(*List)
	if !ok {
		t.Fatal("did not parse as a list")
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected two items, got %+v", list.Items)
	}
	for i, id := range []resource.ID{
		resource.MustParseID("ns:deployment/foo"),
		resource.MustParseID("ns:service/bar")} {
		if list.Items[i].ResourceID() != id {
			t.Errorf("At %d, expected %q, got %q", i, id, list.Items[i].ResourceID())
		}
	}
}

func TestUnmarshalDeploymentList(t *testing.T) {
	doc := `---
kind: DeploymentList
metadata:
  name: list
items:
- kind: Deployment
  metadata:
    name: foo
    namespace: ns
- kind: Deployment
  metadata:
    name: bar
    namespace: ns
`
	res, err := unmarshalObject("", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	list, ok := res.(*List)
	if !ok {
		t.Fatal("did not parse as a list")
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected two items, got %+v", list.Items)
	}
	for i, id := range []resource.ID{
		resource.MustParseID("ns:deployment/foo"),
		resource.MustParseID("ns:deployment/bar")} {
		if list.Items[i].ResourceID() != id {
			t.Errorf("At %d, expected %q, got %q", i, id, list.Items[i].ResourceID())
		}
	}
}

func debyte(r resource.Resource) resource.Resource {
	if res, ok := r.(interface {
		debyte()
	}); ok {
		res.debyte()
	}
	return r
}

func TestLoadSome(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}
	objs, err := Load(dir, []string{dir}, false)
	if err != nil {
		t.Error(err)
	}
	if len(objs) != len(testfiles.ResourceMap) {
		t.Errorf("expected %d objects from %d files, got result:\n%#v", len(testfiles.ResourceMap), len(testfiles.Files), objs)
	}
}

func TestChartTracker(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}

	ct, err := newChartTracker(dir)
	if err != nil {
		t.Fatal(err)
	}

	noncharts := []string{"garbage", "locked-service-deploy.yaml",
		"test", "test/test-service-deploy.yaml"}
	for _, f := range noncharts {
		fq := filepath.Join(dir, f)
		if ct.isDirChart(fq) {
			t.Errorf("%q thought to be a chart", f)
		}
		if f == "garbage" {
			continue
		}
		if m, err := Load(dir, []string{fq}, false); err != nil || len(m) == 0 {
			t.Errorf("Load returned 0 objs, err=%v", err)
		}
	}
	if !ct.isDirChart(filepath.Join(dir, "charts/nginx")) {
		t.Errorf("charts/nginx not recognized as chart")
	}
	if !ct.isPathInChart(filepath.Join(dir, "charts/nginx/Chart.yaml")) {
		t.Errorf("charts/nginx/Chart.yaml not recognized as in chart")
	}

	chartfiles := []string{"charts",
		"charts/nginx",
		"charts/nginx/Chart.yaml",
		"charts/nginx/values.yaml",
		"charts/nginx/templates/deployment.yaml",
	}
	for _, f := range chartfiles {
		fq := filepath.Join(dir, f)
		if m, err := Load(dir, []string{fq}, false); err != nil || len(m) != 0 {
			t.Errorf("%q not ignored as a chart should be", f)
		}
	}

}

func TestLoadSomeWithSopsNoneEncrypted(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}
	objs, err := Load(dir, []string{dir}, true)
	if err != nil {
		t.Error(err)
	}
	if len(objs) != len(testfiles.ResourceMap) {
		t.Errorf("expected %d objects from %d files, got result:\n%#v", len(testfiles.ResourceMap), len(testfiles.Files), objs)
	}
}

func TestLoadSomeWithSopsAllEncrypted(t *testing.T) {
	gpgHome, gpgCleanup := gpgtest.ImportGPGKey(t, testfiles.TestPrivateKey)
	defer gpgCleanup()
	os.Setenv("GNUPGHOME", gpgHome)
	defer os.Unsetenv("GNUPGHOME")

	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	if err := testfiles.WriteSopsEncryptedTestFiles(dir); err != nil {
		t.Fatal(err)
	}
	objs, err := Load(dir, []string{dir}, true)
	if err != nil {
		t.Error(err)
	}
	for expected := range testfiles.EncryptedResourceMap {
		assert.NotNil(t, objs[expected.String()], "expected to find %s in manifest map after decryption", expected)
	}
}
