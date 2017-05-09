package history

import (
	"time"
)

// Entry is the serializable format of an event. It's a
// backwards-compatible-ish shim.
type Entry struct {
	Stamp *time.Time `json:",omitempty"`
	Type  string
	Data  string
	Event *Event `json:",omitempty"`
}
