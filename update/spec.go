package update

import (
	"encoding/json"
	"errors"

	"github.com/weaveworks/flux/policy"
)

const (
	Images = "image"
	Policy = "policy"
)

// A tagged union for all (both) kinds of update. The type is just so
// we know how to decode the rest of the struct.
type Spec struct {
	Type string      `json:"type"`
	Spec interface{} `json:"spec"`
}

func (spec *Spec) UnmarshalJSON(in []byte) error {

	var wire struct {
		Type      string          `json:"type"`
		SpecBytes json.RawMessage `json:"spec"`
	}

	if err := json.Unmarshal(in, &wire); err != nil {
		return err
	}
	spec.Type = wire.Type
	switch wire.Type {
	case Policy:
		var update policy.Updates
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	case Images:
		var update ReleaseSpec
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	default:
		return errors.New("unknown spec type: " + wire.Type)
	}
	return nil
}
