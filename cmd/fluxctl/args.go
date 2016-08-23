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

func parseImageOption(i string) (flux.ImageSpec, error) {
	if i == "" {
		return flux.ImageSpecLatest, nil
	}
	return flux.ParseImageSpec(i), nil
}
