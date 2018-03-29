package update

import (
	"encoding/json"
	"errors"
)

const (
	SpecImages = "image"
	SpecPolicy = "policy"
	SpecAuto   = "auto"
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
	case SpecPolicy:
		var update Policy
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	case SpecImages:
		var update ReleaseSpec
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	case SpecAuto:
		var update Automated
		if err := json.Unmarshal(wire.SpecBytes, &update); err != nil {
			return err
		}
		spec.Spec = update
	default:
		return errors.New("unknown spec type: " + wire.Type)
	}
	return nil
}
