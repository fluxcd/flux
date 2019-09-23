package resource

import (
	"fmt"

	"github.com/fluxcd/flux/pkg/policy"
)

// ChangeForPolicyUpdate evaluates a policy update with respect to a
// workload. The reason this exists at all is that an `Update` can
// include qualified policies, for example "tag all containers"; and
// to make actual changes, we need to examine the workload to which
// it's to be applied.
//
// This also translates policy deletion to empty values (i.e., `""`),
// to make it easy to use as command-line arguments or environment
// variables. When represented in manifests, policies are expected to
// have a non-empty value when present, even if it's `"true"`; so an
// empty value can safely denote deletion.
func ChangesForPolicyUpdate(workload Workload, update PolicyUpdate) (map[string]string, error) {
	add, del := update.Add, update.Remove
	// We may be sent the pseudo-policy `policy.TagAll`, which means
	// apply this filter to all containers. To do so, we need to know
	// what all the containers are.
	if tagAll, ok := update.Add.Get(policy.TagAll); ok {
		add = add.Without(policy.TagAll)
		for _, container := range workload.Containers() {
			if tagAll == policy.PatternAll.String() {
				del = del.Add(policy.TagPrefix(container.Name))
			} else {
				add = add.Set(policy.TagPrefix(container.Name), tagAll)
			}
		}
	}

	result := map[string]string{}
	for pol, val := range add {
		if policy.Tag(pol) && !policy.NewPattern(val).Valid() {
			return nil, fmt.Errorf("invalid tag pattern: %q", val)
		}
		result[string(pol)] = val
	}
	for pol, _ := range del {
		result[string(pol)] = ""
	}
	return result, nil
}

type PolicyUpdates map[ID]PolicyUpdate

type PolicyUpdate struct {
	Add    policy.Set `json:"add"`
	Remove policy.Set `json:"remove"`
}
