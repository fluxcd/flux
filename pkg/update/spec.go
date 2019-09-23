package update

import (
	"encoding/json"
	"errors"

	"github.com/fluxcd/flux/pkg/resource"
)

const (
	Images     = "image"
	Policy     = "policy"
	Auto       = "auto"
	Sync       = "sync"
	Containers = "containers"
)

// How did this update get triggered?
type Cause struct {
	Message string
	User    string
}

// A tagged union for all (both) kinds of update. The type is just so
// we know how to decode the rest of the struct.
type Spec struct {
	Type  string      `json:"type"`
	Cause Cause       `json:"cause"`
	Spec  interface{} `json:"spec"`
}

func (spec *Spec) UnmarshalJSON(in []byte) error {
	var wire struct {
		Type      string          `json:"type"`
		Cause     Cause           `json:"cause"`
		SpecBytes json.RawMessage `json:"spec"`
	}

	if err := json.Unmarshal(in, &wire); err != nil {
		return err
	}
	spec.Type = wire.Type
	spec.Cause = wire.Cause
	switch wire.Type {
	case Policy:
		var update resource.PolicyUpdates
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	case Images:
		var update ReleaseImageSpec
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	case Auto:
		var update Automated
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	case Sync:
		var update ManualSync
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	case Containers:
		var update ReleaseContainersSpec
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	default:
		return errors.New("unknown spec type: " + wire.Type)
	}
	return nil
}
