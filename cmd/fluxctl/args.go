package main

import (
	"github.com/weaveworks/flux"
)

func parseServiceOption(s string) (flux.ServiceSpec, error) {
	if s == "" {
		return flux.ServiceSpecAll, nil
	}
	return flux.ParseServiceSpec(s)
}
