package event

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
	"github.com/pkg/errors"
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

	// This is used to label e.g., commits that we _don't_ consider an event in themselves.
	NoneOfTheAbove = "other"

	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

type EventID int64

type Event struct {
	// ID is a UUID for this event. Will be auto-set when saving if blank.
	ID EventID `json:"id"`

	// Identifiers of workloads affected by this event.
	// TODO: rename to WorkloadIDs after adding versioning.
	ServiceIDs []resource.ID `json:"serviceIDs"`

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

type EventWriter interface {
	// LogEvent records a message in the history.
	LogEvent(Event) error
}

func (e Event) WorkloadIDStrings() []string {
	var strWorkloadIDs []string
	for _, workloadID := range e.ServiceIDs {
		strWorkloadIDs = append(strWorkloadIDs, workloadID.String())
	}
	sort.Strings(strWorkloadIDs)
	return strWorkloadIDs
}

func (e Event) String() string {
	if e.Message != "" {
		return e.Message
	}

	strWorkloadIDs := e.WorkloadIDStrings()
	switch e.Type {
	case EventRelease:
		metadata := e.Metadata.(*ReleaseEventMetadata)
		strImageIDs := metadata.Result.ChangedImages()
		if len(strImageIDs) == 0 {
			strImageIDs = []string{"no image changes"}
		}
		if metadata.Spec.Type == "" || metadata.Spec.Type == ReleaseImageSpecType {
			for _, spec := range metadata.Spec.ReleaseImageSpec.ServiceSpecs {
				if spec == update.ResourceSpecAll {
					strWorkloadIDs = []string{"all workloads"}
					break
				}
			}
		}
		if len(strWorkloadIDs) == 0 {
			strWorkloadIDs = []string{"no workloads"}
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
			strings.Join(strWorkloadIDs, ", "),
			user,
			msg,
		)
	case EventAutoRelease:
		metadata := e.Metadata.(*AutoReleaseEventMetadata)
		strImageIDs := metadata.Result.ChangedImages()
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
		if len(strWorkloadIDs) > 0 {
			svcStr = strings.Join(strWorkloadIDs, ", ")
		}
		return fmt.Sprintf("Commit: %s, %s", shortRevision(metadata.Revision), svcStr)
	case EventSync:
		metadata := e.Metadata.(*SyncEventMetadata)
		revStr := "<no revision>"
		if 0 < len(metadata.Commits) && len(metadata.Commits) <= 2 {
			revStr = shortRevision(metadata.Commits[0].Revision)
		} else if len(metadata.Commits) > 2 {
			revStr = fmt.Sprintf(
				"%s..%s",
				shortRevision(metadata.Commits[len(metadata.Commits)-1].Revision),
				shortRevision(metadata.Commits[0].Revision),
			)
		}
		svcStr := "no workloads changed"
		if len(strWorkloadIDs) > 0 {
			svcStr = strings.Join(strWorkloadIDs, ", ")
		}
		return fmt.Sprintf("Sync: %s, %s", revStr, svcStr)
	case EventAutomate:
		return fmt.Sprintf("Automated: %s", strings.Join(strWorkloadIDs, ", "))
	case EventDeautomate:
		return fmt.Sprintf("Deautomated: %s", strings.Join(strWorkloadIDs, ", "))
	case EventLock:
		return fmt.Sprintf("Locked: %s", strings.Join(strWorkloadIDs, ", "))
	case EventUnlock:
		return fmt.Sprintf("Unlocked: %s", strings.Join(strWorkloadIDs, ", "))
	case EventUpdatePolicy:
		return fmt.Sprintf("Updated policies: %s", strings.Join(strWorkloadIDs, ", "))
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
	return shortRevision(c.Revision)
}

// Commit represents the commit information in a sync event. We could
// use git.Commit, but that would lead to an import cycle, and may
// anyway represent coupling (of an internal API to serialised data)
// that we don't want.
type Commit struct {
	Revision string `json:"revision"`
	Message  string `json:"message"`
}

type ResourceError struct {
	ID    resource.ID
	Path  string
	Error string
}

// SyncEventMetadata is the metadata for when new a commit is synced to the cluster
type SyncEventMetadata struct {
	// for parsing old events; Commits is now used in preference
	Revs    []string `json:"revisions,omitempty"`
	Commits []Commit `json:"commits,omitempty"`
	// Which "kinds" of commit this includes; release, autoreleases,
	// policy changes, and "other" (meaning things we didn't commit
	// ourselves)
	Includes map[string]bool `json:"includes,omitempty"`
	// Per-resource errors
	Errors []ResourceError `json:"errors,omitempty"`
	// `true` if we have no record of having synced before
	InitialSync bool `json:"initialSync,omitempty"`
}

// Account for old events, which used the revisions field rather than commits
func (ev *SyncEventMetadata) UnmarshalJSON(b []byte) error {
	type data SyncEventMetadata
	err := json.Unmarshal(b, (*data)(ev))
	if err != nil {
		return err
	}
	if ev.Commits == nil {
		ev.Commits = make([]Commit, len(ev.Revs))
		for i, rev := range ev.Revs {
			ev.Commits[i].Revision = rev
		}
	}
	return nil
}

type ReleaseEventCommon struct {
	Revision string        // the revision which has the changes for the release
	Result   update.Result `json:"result"`
	// Message of the error if there was one.
	Error string `json:"error,omitempty"`
}

const (
	// ReleaseImageSpecType is a type of release spec when there are update.Images
	ReleaseImageSpecType = "releaseImageSpecType"
	// ReleaseContainersSpecType is a type of release spec when there are update.Containers
	ReleaseContainersSpecType = "releaseContainersSpecType"
)

// ReleaseSpec is a spec for images and containers release
type ReleaseSpec struct {
	// Type is ReleaseImageSpecType or ReleaseContainersSpecType
	// if empty (for previous version), then use ReleaseImageSpecType
	Type                  string
	ReleaseImageSpec      *update.ReleaseImageSpec
	ReleaseContainersSpec *update.ReleaseContainersSpec
}

// IsKindExecute reports whether the release spec s has ReleaseImageSpec or ReleaseImageSpec with Kind execute
// or error if s has invalid Type
func (s ReleaseSpec) IsKindExecute() (bool, error) {
	switch s.Type {
	case ReleaseImageSpecType:
		if s.ReleaseImageSpec != nil && s.ReleaseImageSpec.Kind == update.ReleaseKindExecute {
			return true, nil
		}
	case ReleaseContainersSpecType:
		if s.ReleaseContainersSpec != nil && s.ReleaseContainersSpec.Kind == update.ReleaseKindExecute {
			return true, nil
		}

	default:
		return false, errors.Errorf("unknown release spec type %s", s.Type)
	}
	return false, nil
}

// UnmarshalJSON for old version of spec (update.ReleaseImageSpec) where Type is empty
func (s *ReleaseSpec) UnmarshalJSON(b []byte) error {
	type T ReleaseSpec
	t := (*T)(s)
	if err := json.Unmarshal(b, t); err != nil {
		return err
	}

	switch t.Type {
	case "":
		r := &update.ReleaseImageSpec{}
		if err := json.Unmarshal(b, r); err != nil {
			return err
		}
		s.Type = ReleaseImageSpecType
		s.ReleaseImageSpec = r

	case ReleaseImageSpecType, ReleaseContainersSpecType:
		// all good
	default:
		return errors.New("unknown ReleaseSpec type")
	}
	return nil
}

// ReleaseEventMetadata is the metadata for when workloads(s) are released
type ReleaseEventMetadata struct {
	ReleaseEventCommon
	Spec  ReleaseSpec  `json:"spec"`
	Cause update.Cause `json:"cause"`
}

// AutoReleaseEventMetadata is for when workloads(s) are released
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
