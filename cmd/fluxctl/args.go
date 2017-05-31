package main

import (
	"github.com/weaveworks/flux/update"
)

func parseServiceOption(s string) (update.ServiceSpec, error) {
	if s == "" {
		return update.ServiceSpecAll, nil
	}
	return update.ParseServiceSpec(s)
}
