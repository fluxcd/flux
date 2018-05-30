package kubernetes

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

func TestUpdatePolicies(t *testing.T) {
	for _, c := range []struct {
		name    string
		in, out []string
		update  policy.Update
	}{
		{
			name: "adding annotation with others existing",
			in:   []string{"prometheus.io.scrape", "'false'"},
			out:  []string{"prometheus.io.scrape", "'false'", "flux.weave.works/automated", "'true'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding annotation when already has annotation",
			in:   []string{"flux.weave.works/automated", "'true'"},
			out:  []string{"flux.weave.works/automated", "'true'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding annotation when already has annotation and others",
			in:   []string{"flux.weave.works/automated", "'true'", "prometheus.io.scrape", "'false'"},
			out:  []string{"flux.weave.works/automated", "'true'", "prometheus.io.scrape", "'false'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding first annotation",
			in:   nil,
			out:  []string{"flux.weave.works/automated", "'true'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "add and remove different annotations at the same time",
			in:   []string{"flux.weave.works/automated", "'true'", "prometheus.io.scrape", "'false'"},
			out:  []string{"prometheus.io.scrape", "'false'", "flux.weave.works/locked", "'true'"},
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
			in:   []string{"flux.weave.works/automated", "true", "prometheus.io.scrape", "false"},
			out:  []string{"prometheus.io.scrape", "false"},
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove last annotation",
			in:   []string{"flux.weave.works/automated", "true"},
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
			in:   []string{"prometheus.io.scrape", "false"},
			out:  []string{"prometheus.io.scrape", "false"},
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "multiline",
			in:   []string{"flux.weave.works/locked_msg", "|-\n      first\n      second"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.LockedMsg: "foo"},
			},
		},
		{
			name: "multiline with empty line",
			in:   []string{"flux.weave.works/locked_msg", "|-\n      first\n\n      third"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.LockedMsg: "foo"},
			},
		},
	} {
		caseIn := templToString(t, annotationsTemplate, c.in)
		caseOut := templToString(t, annotationsTemplate, c.out)
		resourceID := flux.MustParseResourceID("default:deployment/nginx")
		out, err := (&Manifests{}).UpdatePolicies([]byte(caseIn), resourceID, c.update)
		if err != nil {
			t.Errorf("[%s] %v", c.name, err)
		} else if string(out) != caseOut {
			t.Errorf("[%s] Did not get expected result:\n\n%s\n\nInstead got:\n\n%s", c.name, caseOut, string(out))
		}
	}
}

var annotationsTemplate = template.Must(template.New("").Parse(`---
apiVersion: extensions/v1beta1
kind: Deployment
metadata: # comment really close to the war zone
  name: nginx{{with .}}
  annotations:{{range .}}
    {{index . 0}}: {{printf "%s" (index . 1)}}{{end}}{{end}}
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

func templToString(t *testing.T, templ *template.Template, data []string) string {
	var pairs [][]string
	for i := 0; i < len(data); i += 2 {
		pairs = append(pairs, []string{data[i], data[i+1]})
	}
	out := &bytes.Buffer{}
	err := templ.Execute(out, pairs)
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}
