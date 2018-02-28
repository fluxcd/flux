package kubernetes

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

var changes = []struct {
	name               string
	existing, expected map[string]string
	update             policy.Update
}{
	{
		name:     "adding annotation with others existing",
		existing: map[string]string{"prometheus.io.scrape": "false"},
		expected: map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
		update: policy.Update{
			Add: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "adding annotation when already has annotation",
		existing: map[string]string{"flux.weave.works/automated": "true"},
		expected: map[string]string{"flux.weave.works/automated": "true"},
		update: policy.Update{
			Add: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "adding annotation when already has annotation and others",
		existing: map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
		expected: map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
		update: policy.Update{
			Add: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "adding first annotation",
		existing: nil,
		expected: map[string]string{"flux.weave.works/automated": "true"},
		update: policy.Update{
			Add: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "add and remove different annotations at the same time",
		existing: map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
		expected: map[string]string{"flux.weave.works/locked": "true", "prometheus.io.scrape": "false"},
		update: policy.Update{
			Add:    policy.Set{policy.Locked: "true"},
			Remove: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "remove overrides add for same key",
		existing: nil,
		expected: nil,
		update: policy.Update{
			Add:    policy.Set{policy.Locked: "true"},
			Remove: policy.Set{policy.Locked: "true"},
		},
	},
	{
		name:     "remove annotation with others existing",
		existing: map[string]string{"flux.weave.works/automated": "true", "prometheus.io.scrape": "false"},
		expected: map[string]string{"prometheus.io.scrape": "false"},
		update: policy.Update{
			Remove: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "remove last annotation",
		existing: map[string]string{"flux.weave.works/automated": "true"},
		expected: nil,
		update: policy.Update{
			Remove: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "remove annotation with no annotations",
		existing: nil,
		expected: nil,
		update: policy.Update{
			Remove: policy.Set{policy.Automated: "true"},
		},
	},
	{
		name:     "remove annotation with only others",
		existing: map[string]string{"prometheus.io.scrape": "false"},
		expected: map[string]string{"prometheus.io.scrape": "false"},
		update: policy.Update{
			Remove: policy.Set{policy.Automated: "true"},
		},
	},
}

func TestUpdatePolicies(t *testing.T) {
	for _, c := range changes {
		id := flux.MakeResourceID("default", "deployment", "nginx")
		caseIn := templToString(t, annotationsTemplate, c.existing)
		caseOut := templToString(t, annotationsTemplate, c.expected)
		out, err := (&Manifests{}).UpdatePolicies([]byte(caseIn), id, c.update)

		if err != nil {
			t.Errorf("[%s] %v", c.name, err)
		} else if string(out) != caseOut {
			t.Errorf("[%s] Did not get expected result:\n\n%s\n\nInstead got:\n\n%s", c.name, caseOut, string(out))
		}

	}
}

func TestUpdateListPolicies(t *testing.T) {
	for _, c := range changes {
		id := flux.MakeResourceID("default", "deployment", "b-deployment")
		listIn := templToString(t, listAnnotationsTemplate, c.existing)
		listOut := templToString(t, listAnnotationsTemplate, c.expected)
		out, err := (&Manifests{}).UpdatePolicies([]byte(listIn), id, c.update)

		if err != nil {
			t.Errorf("[%s] %v", c.name, err)
		}

		if string(out) != listOut {
			t.Errorf("[%s]\nhave:\n%v\nwant:\n%v\n", c.name, string(out), listOut)
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

var listAnnotationsTemplate = template.Must(template.New("").Parse(`---
apiVersion: v1
kind: List
items:
  - apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: a-deployment
      annotations:
        my.annotation: true #should be untouched
    spec:
      template:
        metadata:
          labels:
            name: a-deployment
        spec:
          containers:
          - name: a-container
            image: quay.io/weaveworks/helloworld:master-a000001
  - apiVersion: v1
    kind: Service
    metadata:
      # this is a Service and should not have any annotations!
      name: b-deployment
    spec:
      type: NodePort
      ports:
        - protocol: "TCP"
          port: 30062
          targetPort: 80
          nodePort: 30062
      selector:
        name: a-service
  - apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: b-deployment
    {{with .}}  annotations:{{range $k, $v := .}}
        {{$k}}: {{printf "%q" $v}}{{end}}
    {{end}}spec:
      template:
        metadata:
          labels:
            name: b-deployment
        spec:
          containers:
          - name: b-container
            image: quay.io/weaveworks/helloworld:master-a000001
`))

func templToString(t *testing.T, templ *template.Template, data interface{}) string {
	out := &bytes.Buffer{}
	err := templ.Execute(out, data)
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}
