package v9

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fluxcd/flux/pkg/image"
)

func TestChangeEncoding(t *testing.T) {
	ref, _ := image.ParseRef("docker.io/fluxcd/flux")
	name := ref.Name

	for _, update := range []Change{
		{Kind: GitChange, Source: GitUpdate{URL: "git@github.com:fluxcd/flux"}},
		{Kind: ImageChange, Source: ImageUpdate{Name: name}},
	} {
		bytes, err := json.Marshal(update)
		if err != nil {
			t.Fatal(err)
		}
		var update2 Change
		if err = json.Unmarshal(bytes, &update2); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(update, update2) {
			t.Errorf("unmarshaled != original.\nExpected: %#v\nGot: %#v", update, update2)
		}
	}
}
