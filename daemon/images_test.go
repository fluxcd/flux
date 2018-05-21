package daemon

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

func Test_getTagPattern(t *testing.T) {
	resourceID, err := flux.ParseResourceID("default:deployment/helloworld")
	assert.NoError(t, err)
	container := "helloContainer"

	type args struct {
		services  policy.ResourceMap
		service   flux.ResourceID
		container string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Nil policies",
			args: args{services: nil},
			want: "*",
		},
		{
			name: "No match",
			args: args{services: policy.ResourceMap{}},
			want: "*",
		},
		{
			name: "Match",
			args: args{
				services: policy.ResourceMap{
					resourceID: policy.Set{
						policy.Policy(fmt.Sprintf("tag.%s", container)): "glob:master-*",
					},
				},
				service:   resourceID,
				container: container,
			},
			want: "master-*",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTagPattern(tt.args.services, tt.args.service, tt.args.container); got != tt.want {
				t.Errorf("getTagPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}
