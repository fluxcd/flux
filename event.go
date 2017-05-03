package flux

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// These are all the types of events.
const (
	EventCommit     = "commit"
	EventRelease    = "release"
	EventAutomate   = "automate"
	EventDeautomate = "deautomate"
	EventLock       = "lock"
	EventUnlock     = "unlock"

	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

type EventID int64

type Event struct {
	// ID is a UUID for this event. Will be auto-set when saving if blank.
	ID EventID `json:"id"`

	// ServiceIDs affected by this event.
	ServiceIDs []ServiceID `json:"serviceIDs"`

	// Type is the type of event, usually "release" for now, but could be other
	// things later
	Type string `json:"type"`

	// StartedAt is the time the event began.
	StartedAt time.Time `json:"startedAt"`

	// EndedAt is the time the event ended. For instantaneous events, this will
	// be the same as StartedAt.
	EndedAt time.Time `json:"endedAt"`

	// LogLevel for this event. Used to indicate how important it is.
	// `debug|info|warn|error`
	LogLevel string `json:"logLevel"`

	// Message is a pre-formatted string for errors and other stuff. Included for
	// backwards-compatibility, and is now somewhat unnecessary. Should only be
	// used if metadata is empty.
	Message string `json:"message,omitempty"`

	// Metadata is Event.Type-specific metadata. If an event has no metadata,
	// this will be nil.
	Metadata interface{} `json:"metadata,omitempty"`
}

func (e Event) ServiceIDStrings() []string {
	var strServiceIDs []string
	for _, serviceID := range e.ServiceIDs {
		strServiceIDs = append(strServiceIDs, string(serviceID))
	}
	sort.Strings(strServiceIDs)
	return strServiceIDs
}

func (e Event) String() string {
	if e.Message != "" {
		return e.Message
	}

	strServiceIDs := e.ServiceIDStrings()
	switch e.Type {
	case EventRelease:
		metadata := e.Metadata.(ReleaseEventMetadata)
		strImageIDs := metadata.Release.Result.ImageIDs()
		if len(strImageIDs) == 0 {
			strImageIDs = []string{"no image changes"}
		}
		for _, spec := range metadata.Release.Spec.ServiceSpecs {
			switch spec {
			case ServiceSpecAll:
				strServiceIDs = []string{"all services"}
				break
			case ServiceSpecAutomated:
				strServiceIDs = []string{"automated services"}
				break
			}
		}
		if len(strServiceIDs) == 0 {
			strServiceIDs = []string{"no services"}
		}
		var user string
		if metadata.Release.Spec.Cause.User != "" {
			user = fmt.Sprintf(", by %s", metadata.Release.Spec.Cause.User)
		}
		var msg string
		if metadata.Release.Spec.Cause.Message != "" {
			msg = fmt.Sprintf(", with message %q", metadata.Release.Spec.Cause.Message)
		}
		return fmt.Sprintf(
			"Released: %s to %s%s%s",
			strings.Join(strImageIDs, ", "),
			strings.Join(strServiceIDs, ", "),
			user,
			msg,
		)
	case EventCommit:
		metadata := e.Metadata.(CommitEventMetadata)
		return fmt.Sprintf("Commit: %s, %s", metadata.Revision, strings.Join(strServiceIDs, ", "))
	case EventAutomate:
		return fmt.Sprintf("Automated: %s", strings.Join(strServiceIDs, ", "))
	case EventDeautomate:
		return fmt.Sprintf("Deautomated: %s", strings.Join(strServiceIDs, ", "))
	case EventLock:
		return fmt.Sprintf("Locked: %s", strings.Join(strServiceIDs, ", "))
	case EventUnlock:
		return fmt.Sprintf("Unlocked: %s", strings.Join(strServiceIDs, ", "))
	default:
		return "Unknown event"
	}
}

// CommitEventMetadata is the metadata for when new git commits are created
type CommitEventMetadata struct {
	Revision string `json:"revision"`
}

// ReleaseEventMetadata is the metadata for when service(s) are released
type ReleaseEventMetadata struct {
	// Release points to this release
	Release Release `json:"release"`
	// Message of the error if there was one.
	Error string `json:"error,omitempty"`
}
