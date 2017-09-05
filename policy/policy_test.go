package policy

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSON(t *testing.T) {
	boolPolicy := Set{}
	boolPolicy = boolPolicy.Add(Ignore)
	boolPolicy = boolPolicy.Add(Locked)
	policy := boolPolicy.Set(LockedUser, "user@example.com")

	if !(policy.Contains(Ignore) && policy.Contains(Locked)) {
		t.Errorf("Policy did not include those added")
	}
	if val, ok := policy.Get(LockedUser); !ok || val != "user@example.com" {
		t.Errorf("Policy did not include policy that was set")
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
	if !reflect.DeepEqual(boolPolicy, policy2) {
		t.Errorf("Parsing equivalent list did not preserve policy. Expected:\n%#v\nGot:\n%#v\n", policy, policy2)
	}
}
