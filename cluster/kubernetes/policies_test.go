package kubernetes

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

func TestUpdatePolicies(t *testing.T) {
	for _, c := range []struct {
		name    string
		in, out map[string]string
		update  policy.Update
	}{
		{
			name: "adding annotation with others existing",
			in:   map[string]string{"prometheus.io.scrape": "false"},
			out:  map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding annotation when already has annotation",
			in:   map[string]string{"flux.weave.works/automated": "true"},
			out:  map[string]string{"flux.weave.works/automated": "true"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding annotation when already has annotation and others",
			in:   map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			out:  map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding first annotation",
			in:   nil,
			out:  map[string]string{"flux.weave.works/automated": "true"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "add and remove different annotations at the same time",
			in:   map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			out:  map[string]string{"flux.weave.works/locked": "true", "prometheus.io.scrape": "false"},
			update: policy.Update{
				Add:    policy.Set{policy.Locked: "true"},
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove overrides add for same key",
			in:   nil,
			out:  nil,
			update: policy.Update{
				Add:    policy.Set{policy.Locked: "true"},
				Remove: policy.Set{policy.Locked: "true"},
			},
		},
		{
			name: "remove annotation with others existing",
			in:   map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			out:  map[string]string{"prometheus.io.scrape": "false"},
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove last annotation",
			in:   map[string]string{"flux.weave.works/automated": "true"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove annotation with no annotations",
			in:   nil,
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove annotation with only others",
			in:   map[string]string{"prometheus.io.scrape": "false"},
			out:  map[string]string{"prometheus.io.scrape": "false"},
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "multiline",
			in:   map[string]string{"flux.weave.works/locked_msg": "|-\n      first\n      second"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.LockedMsg: "foo"},
			},
		},
		{
			name: "multiline with empty line",
			in:   map[string]string{"flux.weave.works/locked_msg": "|-\n      first\n\n      third"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.LockedMsg: "foo"},
			},
		},
	} {
		caseIn := templToString(t, annotationsTemplate, c.in)
		caseOut := templToString(t, annotationsTemplate, c.out)
		id := flux.MustParseResourceID("default:deplot/nginx")
		out, err := withFile([]byte(caseIn), func(path string) error {
			return (&Manifests{}).UpdatePolicies(path, id, c.update)
		})
		if err != nil {
			t.Errorf("[%s] %v", c.name, err)
		} else if string(out) != caseOut {
			t.Errorf("[%s] Did not get expected result:\n\n%s\n\nInstead got:\n\n%s", c.name, caseOut, string(out))
		}
	}
}

func withFile(content []byte, rewrite func(path string) error) ([]byte, error) {
	tmp, err := ioutil.TempFile("", "fluxtest")
	if err != nil {
		return nil, err
	}
	tmp.Close() // we don't want this handle
	defer os.Remove(tmp.Name())
	if err = ioutil.WriteFile(tmp.Name(), content, os.FileMode(0666)); err != nil {
		return nil, err
	}
	if err = rewrite(tmp.Name()); err != nil {
		return nil, err
	}
	return ioutil.ReadFile(tmp.Name())
}

var annotationsTemplate = template.Must(template.New("").Parse(`---
apiVersion: extensions/v1beta1
kind: Deployment
metadata: # comment really close to the war zone
  {{with .}}annotations:{{range $k, $v := .}}
    {{$k}}: {{printf "%s" $v}}{{end}}
  {{end}}name: nginx
spec:
  replicas: 1
  template:
    metadata: # comment2
      labels:
        name: nginx
    spec:
      containers:
      - image: nginx  # These keys are purposefully un-sorted.
        name: nginx   # And these comments are testing comments.
        ports:
        - containerPort: 80
`))

func templToString(t *testing.T, templ *template.Template, annotations map[string]string) string {
	for k, v := range annotations {
		// Don't wrap multilines
		if !strings.HasPrefix(v, "|") {
			annotations[k] = fmt.Sprintf("%q", v)
		}
	}
	out := &bytes.Buffer{}
	err := templ.Execute(out, annotations)
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}
