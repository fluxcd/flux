package v9

import (
	"encoding/json"
	"errors"

	"github.com/fluxcd/flux/pkg/image"
)

type ChangeKind string

const (
	GitChange   ChangeKind = "git"
	ImageChange ChangeKind = "image"
)

func (k ChangeKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(k))
}

var ErrUnknownChange = errors.New("unknown kind of change")

type Change struct {
	Kind   ChangeKind  // essentially a type tag
	Source interface{} // what changed
}

func (c *Change) UnmarshalJSON(bs []byte) error {
	type raw struct {
		Kind   ChangeKind
		Source json.RawMessage
	}
	var r raw
	var err error
	if err = json.Unmarshal(bs, &r); err != nil {
		return err
	}
	c.Kind = r.Kind

	switch r.Kind {
	case GitChange:
		var git GitUpdate
		err = json.Unmarshal(r.Source, &git)
		c.Source = git
	case ImageChange:
		var image ImageUpdate
		err = json.Unmarshal(r.Source, &image)
		c.Source = image
	default:
		return ErrUnknownChange
	}
	return err
}

type ImageUpdate struct {
	Name image.Name
}

type GitUpdate struct {
	URL, Branch string
}
