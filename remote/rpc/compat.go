package rpc

import (
	"fmt"

	"github.com/weaveworks/flux/update"
)

func requireServiceSpecKinds(ss update.ResourceSpec, kinds []string) error {
	id, err := ss.AsID()
	if err != nil {
		return nil
	}

	_, kind, _ := id.Components()
	if !contains(kinds, kind) {
		return fmt.Errorf("Unsupported resource kind: %s", kind)
	}

	return nil
}

func requireSpecKinds(s update.Spec, kinds []string) error {
	switch s := s.Spec.(type) {
	case update.Policy:
		for id, _ := range s {
			_, kind, _ := id.Components()
			if !contains(kinds, kind) {
				return fmt.Errorf("Unsupported resource kind: %s", kind)
			}
		}
	case update.ReleaseSpec:
		for _, ss := range s.ServiceSpecs {
			if err := requireServiceSpecKinds(ss, kinds); err != nil {
				return err
			}
		}
		for _, id := range s.Excludes {
			_, kind, _ := id.Components()
			if !contains(kinds, kind) {
				return fmt.Errorf("Unsupported resource kind: %s", kind)
			}
		}
	}
	return nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
