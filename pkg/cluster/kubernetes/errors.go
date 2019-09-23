package kubernetes

import (
	"fmt"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
)

func ObjectMissingError(obj string, err error) *fluxerr.Error {
	return &fluxerr.Error{
		Type: fluxerr.Missing,
		Err:  err,
		Help: fmt.Sprintf(`Cluster object %q not found

The object requested was not found in the cluster. Check spelling and
perhaps verify its presence using kubectl.
`, obj)}
}

func UpdateNotSupportedError(kind string) *fluxerr.Error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  fmt.Errorf("updating resource kind %q not supported", kind),
		Help: `Flux does not support updating ` + kind + ` resources.

This may be because those resources do not use images, you are trying
to use a YAML dot notation path annotation for a non HelmRelease
resource, or because it is a new kind of resource in Kubernetes, and
Flux does not support it yet.

If you can use a Deployment instead, Flux can work with
those. Otherwise, you may have to update the resource manually (e.g.,
using kubectl).
`,
	}
}
