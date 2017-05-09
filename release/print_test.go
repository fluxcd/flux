package release

import (
	"bytes"
	"testing"

	"github.com/weaveworks/flux"
)

func TestPrintResults(t *testing.T) {
	for _, example := range []struct {
		name     string
		result   flux.ReleaseResult
		verbose  bool
		expected string
	}{
		{
			name: "basic, just results",
			result: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSuccess,
					Error:  "",
					PerContainer: []flux.ContainerUpdate{
						{
							Container: "helloworld",
							Current:   flux.ImageID{"quay.io", "weaveworks", "helloworld", "master-a000002"},
							Target:    flux.ImageID{"quay.io", "weaveworks", "helloworld", "master-a000001"},
						},
					},
				},
			},
			expected: `
SERVICE             STATUS   UPDATES
default/helloworld  success  helloworld: quay.io/weaveworks/helloworld:master-a000002 -> master-a000001
`,
		},

		{
			name: "With an error, *and* results",
			result: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSuccess,
					Error:  "test error",
					PerContainer: []flux.ContainerUpdate{
						{
							Container: "helloworld",
							Current:   flux.ImageID{"quay.io", "weaveworks", "helloworld", "master-a000002"},
							Target:    flux.ImageID{"quay.io", "weaveworks", "helloworld", "master-a000001"},
						},
					},
				},
			},
			expected: `
SERVICE             STATUS   UPDATES
default/helloworld  success  test error
                             helloworld: quay.io/weaveworks/helloworld:master-a000002 -> master-a000001
`,
		},

		{
			name: "Service results should be sorted",
			result: flux.ReleaseResult{
				flux.ServiceID("d"): flux.ServiceResult{Status: flux.ReleaseStatusSuccess},
				flux.ServiceID("c"): flux.ServiceResult{Status: flux.ReleaseStatusSuccess},
				flux.ServiceID("b"): flux.ServiceResult{Status: flux.ReleaseStatusSuccess},
				flux.ServiceID("a"): flux.ServiceResult{Status: flux.ReleaseStatusSuccesSuccess},
			},
			expected: `
SERVICE   STATUS   UPDATES
a         success  
b         success  
c         success  
d         success  
`,
		},
	} {
		out := &bytes.Buffer{}
		out.WriteString("\n") // All our "expected" values start with a newline, to make maintaining them easier.
		PrintResults(out, example.result, example.verbose)
		if out.String() != example.expected {
			t.Errorf(
				"Name: %s\nPrintResults(out, %#v, %v)\nExpected\n-------%s-------\nGot\n-------%s-------",
				example.name,
				example.result,
				example.verbose,
				example.expected,
				out.String(),
			)
		}
	}
}
