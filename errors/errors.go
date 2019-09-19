package errors

import (
	"encoding/json"
	"errors"
)

// Representation of errors in the API. These are divided into a small
// number of categories, essentially distinguished by whose fault the
// error is; i.e., is this error:
//  - a transient problem with the service, so worth trying again?
//  - not going to work until the user takes some other action, e.g., updating config?
type Error struct {
	Type Type
	// a message that can be printed out for the user
	Help string `json:"help"`
	// the underlying error that can be e.g., logged for developers to look at
	Err error
}

func (e *Error) Error() string {
	return e.Err.Error()
}

type Type string

const (
	// The operation looked fine on paper, but something went wrong
	Server Type = "server"
	// The thing you mentioned, whatever it is, just doesn't exist
	Missing = "missing"
	// The operation was well-formed, but you asked for something that
	// can't happen at present (e.g., because you've not supplied some
	// config yet)
	User = "user"
)

func IsMissing(err error) bool {
	if err, ok := err.(*Error); ok && err.Type == Missing {
		return true
	}
	return false
}

func (e *Error) MarshalJSON() ([]byte, error) {
	var errMsg string
	if e.Err != nil {
		errMsg = e.Err.Error()
	}
	jsonable := &struct {
		Type string `json:"type"`
		Help string `json:"help"`
		Err  string `json:"error,omitempty"`
	}{
		Type: string(e.Type),
		Help: e.Help,
		Err:  errMsg,
	}
	return json.Marshal(jsonable)
}

func (e *Error) UnmarshalJSON(data []byte) error {
	jsonable := &struct {
		Type string `json:"type"`
		Help string `json:"help"`
		Err  string `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal(data, &jsonable); err != nil {
		return err
	}
	e.Type = Type(jsonable.Type)
	e.Help = jsonable.Help
	if jsonable.Err != "" {
		e.Err = errors.New(jsonable.Err)
	}
	return nil
}

func CoverAllError(err error) *Error {
	return &Error{
		Type: User,
		Err:  err,
		Help: `Error: ` + err.Error() + `

We don't have a specific help message for the error above.

It would help us remedy this if you log an issue at

    https://github.com/fluxcd/flux/issues

saying what you were doing when you saw this, and quoting the message
at the top.
`,
	}
}
