package update

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

type Policy map[flux.ResourceID]PolicyChange

type PolicyChange struct {
	Add    policy.Set `json:"add"`
	Remove policy.Set `json:"remove"`
}

const (
	LabelAutomated     = "Automated"
	LabelDeautomated   = "Deautomated"
	LabelLocked        = "Locked"
	LabelUnlocked      = "Unlocked"
	LabelUpdatedPolicy = "Updated policy" // catch-all
)

func (p Policy) CommitMessage(cause Cause) string {
	summary := summarisePolicy(p)
	commitMsg := &bytes.Buffer{}
	prefix := "- "
	switch {
	case cause.Message != "":
		fmt.Fprintf(commitMsg, "%s\n\n", cause.Message)
	case len(summary) > 1:
		fmt.Fprintf(commitMsg, "Updated policies\n\n")
	default:
		prefix = ""
	}

	for label, ids := range summary {
		fmt.Fprintf(commitMsg, "%s%s: %s\n", prefix, label, strings.Join(ids.ToStringSlice(), ", "))
	}
	return commitMsg.String()
}

// summarisePolicy builds a map of type->serviceID, for the updates
// given.
func summarisePolicy(updates Policy) map[string]flux.ResourceIDSet {
	byType := map[string]flux.ResourceIDSet{}
	addID := func(t string, id flux.ResourceID) {
		s := flux.ResourceIDSetOf(id)
		byType[t] = s.With(byType[t])
	}

	for serviceID, u := range updates {
		for p, _ := range u.Add {
			switch {
			case p == policy.Automated:
				addID(LabelAutomated, serviceID)
			case p == policy.Locked:
				addID(LabelLocked, serviceID)
			default:
				addID(LabelUpdatedPolicy, serviceID)
			}
		}
		for p, _ := range u.Remove {
			switch {
			case p == policy.Automated:
				addID(LabelDeautomated, serviceID)
			case p == policy.Locked:
				addID(LabelUnlocked, serviceID)
			default:
				addID(LabelUpdatedPolicy, serviceID)
			}
		}
	}
	return byType
}
