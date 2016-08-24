package main

import (
	"github.com/weaveworks/fluxy"
)

func parseServiceOption(s string) (flux.ServiceSpec, error) {
	if s == "" {
		return flux.ServiceSpecAll, nil
	}
	return flux.ParseServiceSpec(s)
}
