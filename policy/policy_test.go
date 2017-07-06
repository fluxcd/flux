package policy

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSON(t *testing.T) {
	policy := Set{}
	policy = policy.Add(Ignore)
	policy = policy.Add(Locked)

	if !(policy.Contains(Ignore) && policy.Contains(Locked)) {
		t.Errorf("Policy did not include those added")
	}

	bs, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}

	var policy2 Set
	if err = json.Unmarshal(bs, &policy2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(policy, policy2) {
		t.Errorf("Roundtrip did not preserve policy. Expected:\n%#v\nGot:\n%#v\n", policy, policy2)
	}

	listyPols := []Policy{Ignore, Locked}
	bs, err = json.Marshal(listyPols)
	if err != nil {
		t.Fatal(err)
	}
	policy2 = Set{}
	if err = json.Unmarshal(bs, &policy2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(policy, policy2) {
		t.Errorf("Parsing equivalent list did not preserve policy. Expected:\n%#v\nGot:\n%#v\n", policy, policy2)
	}
}
