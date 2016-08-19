package flux

import "errors"

func fileFor(path string, id ServiceID) (filename string, err error) {
	// Use kubeservice to identify the resource definition file for the given
	// service ID.
	return "", errors.New("fileFor not implemented")
}
