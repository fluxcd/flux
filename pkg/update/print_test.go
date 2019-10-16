package update

import (
	"bytes"
	"testing"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

func mustParseRef(s string) image.Ref {
	ref, err := image.ParseRef(s)
	if err != nil {
		panic(err)
	}
	return ref
}

func TestPrintResults(t *testing.T) {
	for _, example := range []struct {
		name      string
		result    Result
		verbosity int
		expected  string
	}{
		{
			name: "basic, just results",
			result: Result{
				resource.MustParseID("default/helloworld"): WorkloadResult{
					Status: ReleaseStatusSuccess,
					Error:  "",
					PerContainer: []ContainerUpdate{
						{
							Container: "helloworld",
							Current:   mustParseRef("quay.io/weaveworks/helloworld:master-a000002"),
							Target:    mustParseRef("quay.io/weaveworks/helloworld:master-a000001"),
						},
					},
				},
			},
			expected: `
WORKLOAD            STATUS   UPDATES
default/helloworld  success  helloworld: quay.io/weaveworks/helloworld:master-a000002 -> master-a000001
`,
		},

		{
			name: "With an error, *and* results",
			result: Result{
				resource.MustParseID("default/helloworld"): WorkloadResult{
					Status: ReleaseStatusSuccess,
					Error:  "test error",
					PerContainer: []ContainerUpdate{
						{
							Container: "helloworld",
							Current:   mustParseRef("quay.io/weaveworks/helloworld:master-a000002"),
							Target:    mustParseRef("quay.io/weaveworks/helloworld:master-a000001"),
						},
					},
				},
			},
			expected: `
WORKLOAD            STATUS   UPDATES
default/helloworld  success  test error
                             helloworld: quay.io/weaveworks/helloworld:master-a000002 -> master-a000001
`,
		},

		{
			name: "Service results should be sorted",
			result: Result{
				resource.MustParseID("default/d"): WorkloadResult{Status: ReleaseStatusSuccess},
				resource.MustParseID("default/c"): WorkloadResult{Status: ReleaseStatusSuccess},
				resource.MustParseID("default/b"): WorkloadResult{Status: ReleaseStatusSuccess},
				resource.MustParseID("default/a"): WorkloadResult{Status: ReleaseStatusSuccess},
			},
			expected: `
WORKLOAD   STATUS   UPDATES
default/a  success  
default/b  success  
default/c  success  
default/d  success  
`,
		},
	} {
		out := &bytes.Buffer{}
		out.WriteString("\n") // All our "expected" values start with a newline, to make maintaining them easier.
		PrintResults(out, example.result, example.verbosity)
		if out.String() != example.expected {
			t.Errorf(
				"Name: %s\nPrintResults(out, %#v, %v)\nExpected\n-------%s-------\nGot\n-------%s-------",
				example.name,
				example.result,
				example.verbosity,
				example.expected,
				out.String(),
			)
		}
	}
}
