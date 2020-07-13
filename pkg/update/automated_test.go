package update

import (
	"testing"

	"github.com/fluxcd/flux/pkg/resource"
)

func TestCommitMessage(t *testing.T) {
	automated := Automated{}
	result := Result{
		resource.MakeID("ns", "kind", "1"): {
			Status: ReleaseStatusSuccess,
			PerContainer: []ContainerUpdate{
				{Target: mustParseRef("docker.io/image:v1")},
				{Target: mustParseRef("docker.io/image:v2")},
				{Target: mustParseRef("docker.io/image:v3")},
				{Target: mustParseRef("docker.io/image:v4")},
				{Target: mustParseRef("docker.io/image:v5")},
				{Target: mustParseRef("docker.io/image:v6")},
				{Target: mustParseRef("docker.io/image:v7")},
				{Target: mustParseRef("docker.io/image:v8")},
				{Target: mustParseRef("docker.io/image:v9")},
				{Target: mustParseRef("docker.io/image:v10")},
				{Target: mustParseRef("docker.io/image:v11")},
			},
		},
	}
	result.ChangedImages()

	actual := automated.CommitMessage(result)
	expected := `Auto-release multiple (11) images

 - docker.io/image:v1
 - docker.io/image:v10
 - docker.io/image:v11
 - docker.io/image:v2
 - docker.io/image:v3
 - docker.io/image:v4
 - docker.io/image:v5
 - docker.io/image:v6
 - docker.io/image:v7
 - docker.io/image:v8
 - docker.io/image:v9
`
	if actual != expected {
		t.Fatalf("Expected git commit message: '%s', was '%s'", expected, actual)
	}
}
