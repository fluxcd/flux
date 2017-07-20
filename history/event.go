package history

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"encoding/json"
	"errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/update"
)

// These are all the types of events.
const (
	EventCommit       = "commit"
	EventSync         = "sync"
	EventRelease      = "release"
	EventAutoRelease  = "autorelease"
	EventAutomate     = "automate"
	EventDeautomate   = "deautomate"
	EventLock         = "lock"
	EventUnlock       = "unlock"
	EventUpdatePolicy = "update_policy"

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
	ServiceIDs []flux.ServiceID `json:"serviceIDs"`

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
	Metadata EventMetadata `json:"metadata,omitempty"`
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
		metadata := e.Metadata.(*ReleaseEventMetadata)
		strImageIDs := metadata.Result.ImageIDs()
		if len(strImageIDs) == 0 {
			strImageIDs = []string{"no image changes"}
		}
		for _, spec := range metadata.Spec.ServiceSpecs {
			if spec == update.ServiceSpecAll {
				strServiceIDs = []string{"all services"}
				break
			}
		}
		if len(strServiceIDs) == 0 {
			strServiceIDs = []string{"no services"}
		}
		var user string
		if metadata.Cause.User != "" {
			user = fmt.Sprintf(", by %s", metadata.Cause.User)
		}
		var msg string
		if metadata.Cause.Message != "" {
			msg = fmt.Sprintf(", with message %q", metadata.Cause.Message)
		}
		return fmt.Sprintf(
			"Released: %s to %s%s%s",
			strings.Join(strImageIDs, ", "),
			strings.Join(strServiceIDs, ", "),
			user,
			msg,
		)
	case EventAutoRelease:
		metadata := e.Metadata.(*AutoReleaseEventMetadata)
		strImageIDs := metadata.Result.ImageIDs()
		if len(strImageIDs) == 0 {
			strImageIDs = []string{"no image changes"}
		}
		return fmt.Sprintf(
			"Automated release of %s",
			strings.Join(strImageIDs, ", "),
		)
	case EventCommit:
		metadata := e.Metadata.(*CommitEventMetadata)
		svcStr := "<no changes>"
		if len(strServiceIDs) > 0 {
			svcStr = strings.Join(strServiceIDs, ", ")
		}
		return fmt.Sprintf("Commit: %s, %s", shortRevision(metadata.Revision), svcStr)
	case EventSync:
		metadata := e.Metadata.(*SyncEventMetadata)
		revStr := "<no revision>"
		if 0 < len(metadata.Revisions) && len(metadata.Revisions) <= 2 {
			revStr = shortRevision(metadata.Revisions[0])
		} else if len(metadata.Revisions) > 2 {
			revStr = fmt.Sprintf(
				"%s..%s",
				shortRevision(metadata.Revisions[len(metadata.Revisions)-1]),
				shortRevision(metadata.Revisions[0]),
			)
		}
		svcStr := "<no changes>"
		if len(strServiceIDs) > 0 {
			svcStr = strings.Join(strServiceIDs, ", ")
		}
		return fmt.Sprintf("Sync: %s, %s", revStr, svcStr)
	case EventAutomate:
		return fmt.Sprintf("Automated: %s", strings.Join(strServiceIDs, ", "))
	case EventDeautomate:
		return fmt.Sprintf("Deautomated: %s", strings.Join(strServiceIDs, ", "))
	case EventLock:
		return fmt.Sprintf("Locked: %s", strings.Join(strServiceIDs, ", "))
	case EventUnlock:
		return fmt.Sprintf("Unlocked: %s", strings.Join(strServiceIDs, ", "))
	default:
		return fmt.Sprintf("Unknown event: %s", e.Type)
	}
}

func shortRevision(rev string) string {
	if len(rev) <= 7 {
		return rev
	}
	return rev[:7]
}

// CommitEventMetadata is the metadata for when new git commits are created
type CommitEventMetadata struct {
	Revision string        `json:"revision,omitempty"`
	Spec     *update.Spec  `json:"spec"`
	Result   update.Result `json:"result,omitempty"`
}

func (c CommitEventMetadata) ShortRevision() string {
	if len(c.Revision) <= 7 {
		return c.Revision
	}
	return c.Revision[:7]
}

// SyncEventMetadata is the metadata for when new a commit is synced to the cluster
type SyncEventMetadata struct {
	Revisions []string `json:"revisions,omitempty"`
}

type ReleaseEventCommon struct {
	Revision string        // the revision which has the changes for the release
	Result   update.Result `json:"result"`
	// Message of the error if there was one.
	Error string `json:"error,omitempty"`
}

// ReleaseEventMetadata is the metadata for when service(s) are released
type ReleaseEventMetadata struct {
	ReleaseEventCommon
	Spec  update.ReleaseSpec `json:"spec"`
	Cause update.Cause       `json:"cause"`
}

// AutoReleaseEventMetadata is for when service(s) are released
// automatically because there's a new image or images
type AutoReleaseEventMetadata struct {
	ReleaseEventCommon
	Spec update.Automated `json:"spec"`
}

type UnknownEventMetadata map[string]interface{}

func (e *Event) UnmarshalJSON(in []byte) error {
	type alias Event
	var wireEvent struct {
		*alias
		MetadataBytes json.RawMessage `json:"metadata,omitempty"`
	}
	wireEvent.alias = (*alias)(e)

	// Now unmarshall custom wireEvent with RawMessage
	if err := json.Unmarshal(in, &wireEvent); err != nil {
		return err
	}
	if wireEvent.Type == "" {
		return errors.New("Event type is empty")
	}

	// The cases correspond to kinds of event that we care about
	// processing e.g., for notifications.
	switch wireEvent.Type {
	case EventRelease:
		var metadata ReleaseEventMetadata
		if err := json.Unmarshal(wireEvent.MetadataBytes, &metadata); err != nil {
			return err
		}
		e.Metadata = &metadata
		break
	case EventAutoRelease:
		var metadata AutoReleaseEventMetadata
		if err := json.Unmarshal(wireEvent.MetadataBytes, &metadata); err != nil {
			return err
		}
		e.Metadata = &metadata
		break
	case EventCommit:
		var metadata CommitEventMetadata
		if err := json.Unmarshal(wireEvent.MetadataBytes, &metadata); err != nil {
			return err
		}
		e.Metadata = &metadata
		break
	case EventSync:
		var metadata SyncEventMetadata
		if err := json.Unmarshal(wireEvent.MetadataBytes, &metadata); err != nil {
			return err
		}
		e.Metadata = &metadata
		break
	default:
		if len(wireEvent.MetadataBytes) > 0 {
			var metadata UnknownEventMetadata
			if err := json.Unmarshal(wireEvent.MetadataBytes, &metadata); err != nil {
				return err
			}
			e.Metadata = metadata
		}
	}

	// By default, leave the Event Metadata as map[string]interface{}
	return nil
}

// EventMetadata is a type safety trick used to make sure that Metadata field
// of Event is always a pointer, so that consumers can cast without being
// concerned about encountering a value type instead. It works by virtue of the
// fact that the method is only defined for pointer receivers; the actual
// method chosen is entirely arbitary.
type EventMetadata interface {
	Type() string
}

func (cem *CommitEventMetadata) Type() string {
	return EventCommit
}

func (cem *SyncEventMetadata) Type() string {
	return EventSync
}

func (rem *ReleaseEventMetadata) Type() string {
	return EventRelease
}

func (rem *AutoReleaseEventMetadata) Type() string {
	return EventAutoRelease
}

// Special exception from pointer receiver rule, as UnknownEventMetadata is a
// type alias for a map
func (uem UnknownEventMetadata) Type() string {
	return "unknown"
}
