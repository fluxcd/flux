package kubernetes

import (
	"errors"
	"fmt"

	fluxerr "github.com/weaveworks/flux/errors"
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

var ErrReplicationControllersDeprecated = &fluxerr.Error{
	Type: fluxerr.User,
	Err:  errors.New("updating replication controllers is deprecated"),
	Help: `Using Flux to update replication controllers is deprecated.

ReplicationController resources are difficult to update, and it is
almost certainly better to use a Deployment resource instead. Please
see

    https://kubernetes.io/docs/user-guide/replication-controller/#deployment-recommended

If replacing with a Deployment is not possible, you can still update a
ReplicationController manually (e.g., with kubectl rolling-update).
`,
}

func UpdateNotSupportedError(kind string) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  fmt.Errorf("updating resource kind %q not supported", kind),
		Help: `Flux does not support updating ` + kind + ` resources.

This may be because those resources do not use images, or because it
is a new kind of resource in Kubernetes, and Flux does not support it
yet.

If you can use a Deployment instead, Flux can work with
those. Otherwise, you may have to update the resource manually (e.g.,
using kubectl).
`,
	}
}
