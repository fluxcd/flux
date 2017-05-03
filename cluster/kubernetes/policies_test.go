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
		in, out map[string]string
		update  flux.PolicyUpdate
	}{
		{
			name: "adding annotation with others existing",
			in:   map[string]string{"prometheus.io.scrape": "false"},
			out:  map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			update: flux.PolicyUpdate{
				Add: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "adding annotation when already has annotation",
			in:   map[string]string{"flux.weave.works/automated": "true"},
			out:  map[string]string{"flux.weave.works/automated": "true"},
			update: flux.PolicyUpdate{
				Add: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "adding annotation when already has annotation and others",
			in:   map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			out:  map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			update: flux.PolicyUpdate{
				Add: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "adding first annotation",
			in:   nil,
			out:  map[string]string{"flux.weave.works/automated": "true"},
			update: flux.PolicyUpdate{
				Add: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "add and remove different annotations at the same time",
			in:   map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			out:  map[string]string{"flux.weave.works/locked": "true", "prometheus.io.scrape": "false"},
			update: flux.PolicyUpdate{
				Add:    []flux.Policy{policy.Locked},
				Remove: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "remove overrides add for same key",
			in:   nil,
			out:  nil,
			update: flux.PolicyUpdate{
				Add:    []flux.Policy{policy.Locked},
				Remove: []flux.Policy{policy.Locked},
			},
		},
		{
			name: "remove annotation with others existing",
			in:   map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
			out:  map[string]string{"prometheus.io.scrape": "false"},
			update: flux.PolicyUpdate{
				Remove: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "remove last annotation",
			in:   map[string]string{"flux.weave.works/automated": "true"},
			out:  nil,
			update: flux.PolicyUpdate{
				Remove: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "remove annotation with no annotations",
			in:   nil,
			out:  nil,
			update: flux.PolicyUpdate{
				Remove: []flux.Policy{policy.Automated},
			},
		},
		{
			name: "remove annotation with only others",
			in:   map[string]string{"prometheus.io.scrape": "false"},
			out:  map[string]string{"prometheus.io.scrape": "false"},
			update: flux.PolicyUpdate{
				Remove: []flux.Policy{policy.Automated},
			},
		},
	} {
		caseIn := templToString(t, annotationsTemplate, c.in)
		caseOut := templToString(t, annotationsTemplate, c.out)
		out, err := (&Cluster{}).UpdatePolicies([]byte(caseIn), c.update)
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
  {{with .}}annotations:{{range $k, $v := .}}
    {{$k}}: {{printf "%q" $v}}{{end}}
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

func templToString(t *testing.T, templ *template.Template, data interface{}) string {
	out := &bytes.Buffer{}
	err := templ.Execute(out, data)
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}
