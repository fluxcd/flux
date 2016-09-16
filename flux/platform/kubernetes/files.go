package kubernetes

import (
	"errors"
	"io"

	"github.com/weaveworks/fluxy/flux"
)

// UpdatePodController takes the body of a ReplicationController or Deployment
// resource definition (specified in YAML) and the name of the new image that
// should be put in the definition (in the format "repo.org/group/name:tag"). It
// returns a new resource definition body where all references to the old image
// have been replaced with the new one.
//
// This function has many additional requirements that are likely in flux. Read
// the source to learn about them.
func UpdatePodController(buf []byte, newImageName string, trace io.Writer) ([]byte, error) {
	return nil, errors.New("not implemented")
}

// PodControllersFor recursively searches under path for the pod controller
// files which are responsible for driving the given service. It presumes
// kubeservice is available in the PWD or PATH.
func PodControllersFor(path string, serviceID flux.ServiceID) (filenames []string, err error) {
	return nil, errors.New("not implemented")
}
