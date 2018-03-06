package kubernetes

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
)

// updatePodController takes the body of a resource definition
// (specified in YAML), the ID of a particular resource and container
// therein, and the name of the new image that should be put in the
// definition (in the format "repo.org/group/name:tag") for that
// resource and container. It returns a new resource definition body
// where all references to the old image have been replaced with the
// new one.
//
// This function has many additional requirements that are likely in flux. Read
// the source to learn about them.
func updatePodController(file []byte, resource flux.ResourceID, container string, newImageID image.Ref) ([]byte, error) {
	namespace, kind, name := resource.Components()
	if _, ok := resourceKinds[strings.ToLower(kind)]; !ok {
		return nil, UpdateNotSupportedError(kind)
	}

	args := []string{"image", "--namespace", namespace, "--kind", kind, "--name", name}
	args = append(args, "--container", container, "--image", newImageID.String())

	println("TRACE:", "kubeyaml", strings.Join(args, " "))
	cmd := exec.Command("kubeyaml", args...)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.Stdin = bytes.NewBuffer(file)
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return nil, errors.New(strings.TrimSpace(errOut.String()))
	}
	return out.Bytes(), nil
}
