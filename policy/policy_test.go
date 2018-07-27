package policy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/flux"
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

func Test_GetTagPattern(t *testing.T) {
	resourceID, err := flux.ParseResourceID("default:deployment/helloworld")
	assert.NoError(t, err)
	container := "helloContainer"

	type args struct {
		services  ResourceMap
		service   flux.ResourceID
		container string
	}
	tests := []struct {
		name string
		args args
		want Pattern
	}{
		{
			name: "Nil policies",
			args: args{services: nil},
			want: PatternAll,
		},
		{
			name: "No match",
			args: args{services: ResourceMap{}},
			want: PatternAll,
		},
		{
			name: "Match",
			args: args{
				services: ResourceMap{
					resourceID: Set{
						Policy(fmt.Sprintf("tag.%s", container)): "glob:master-*",
					},
				},
				service:   resourceID,
				container: container,
			},
			want: NewPattern("master-*"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTagPattern(tt.args.services, tt.args.service, tt.args.container); got != tt.want {
				t.Errorf("GetTagPattern() = %v, want %v", got, tt.want)

			}
		})
	}
}
