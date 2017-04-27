package flux

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	PolicyNone      = Policy("")
	PolicyLocked    = Policy("locked")
	PolicyAutomated = Policy("automated")
)

// Policy is an string, denoting the current deployment policy of a service,
// e.g. automated, or locked.
type Policy string

func ParsePolicy(s string) Policy {
	for _, p := range []Policy{
		PolicyLocked,
		PolicyAutomated,
	} {
		if s == string(p) {
			return p
		}
	}
	return PolicyNone
}

type PolicyUpdates map[ServiceID]PolicyUpdate

// Events builds a map of events (by type), for all the events in this set of
// updates. There will be one event per type, containing all service ids
// affected by that event. e.g. all automated services will share an event.
func (us PolicyUpdates) Events(now time.Time) map[string]Event {
	eventsByType := map[string]Event{}
	for serviceID, update := range us {
		for _, eventType := range update.EventTypes() {
			e, ok := eventsByType[eventType]
			if !ok {
				e = Event{
					ServiceIDs: []ServiceID{},
					Type:       eventType,
					StartedAt:  now,
					EndedAt:    now,
					LogLevel:   LogLevelInfo,
				}
			}
			e.ServiceIDs = append(e.ServiceIDs, serviceID)
			eventsByType[eventType] = e
		}
	}
	return eventsByType
}

func (u PolicyUpdates) CommitMessage(now time.Time) string {
	events := u.Events(now)
	commitMsg := &bytes.Buffer{}
	prefix := ""
	if len(events) > 1 {
		fmt.Fprintf(commitMsg, "Updated service policies:\n\n")
		prefix = "- "
	}
	for _, event := range events {
		fmt.Fprintf(commitMsg, "%s%v\n", prefix, event)
	}
	return commitMsg.String()
}

type PolicyUpdate struct {
	Add    []Policy `json:"add"`
	Remove []Policy `json:"remove"`
}

// EventTypes is a deduped list of all event types this update contains
func (u PolicyUpdate) EventTypes() []string {
	types := map[string]struct{}{}
	for _, p := range u.Add {
		switch p {
		case PolicyAutomated:
			types[EventAutomate] = struct{}{}
		case PolicyLocked:
			types[EventLock] = struct{}{}
		}
	}

	for _, p := range u.Remove {
		switch p {
		case PolicyAutomated:
			types[EventDeautomate] = struct{}{}
		case PolicyLocked:
			types[EventUnlock] = struct{}{}
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}

type PolicySet []Policy

func (s PolicySet) String() string {
	var ps []string
	for _, p := range s {
		ps = append(ps, string(p))
	}
	return "{" + strings.Join(ps, ", ") + "}"
}

func (s PolicySet) Add(ps ...Policy) PolicySet {
	dedupe := map[Policy]struct{}{}
	for _, p := range s {
		dedupe[p] = struct{}{}
	}
	for _, p := range ps {
		dedupe[p] = struct{}{}
	}
	var result PolicySet
	for p := range dedupe {
		result = append(result, p)
	}
	return result
}

func (s PolicySet) Contains(needle Policy) bool {
	for _, p := range s {
		if p == needle {
			return true
		}
	}
	return false
}
