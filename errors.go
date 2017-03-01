package flux

import (
	"encoding/json"
	"errors"
)

type HelpfulError interface {
	Base() *BaseError
}

// Representation of errors in the API. These are divided into a small
// number of categories, essentially distinguished by whose fault the
// error is; i.e., is this error:
//  - a transient problem with the service, so worth trying again?
//  - not going to work until the user takes some other action, e.g., updating config?

type BaseError struct {
	// a message that can be printed out for the user
	Help string `json:"help"`
	// the underlying error that can be e.g., logged for developers to look at
	Err error
}

func (e *BaseError) Base() *BaseError {
	return e
}

func (e *BaseError) Error() string {
	return e.Err.Error()
}

func (e *BaseError) MarshalJSON() ([]byte, error) {
	var errMsg string
	if e.Err != nil {
		errMsg = e.Err.Error()
	}
	jsonable := &struct {
		Help string `json:"help"`
		Err  string `json:"error,omitempty"`
	}{
		Help: e.Help,
		Err:  errMsg,
	}
	return json.Marshal(jsonable)
}

func (e *BaseError) UnmarshalJSON(data []byte) error {
	jsonable := &struct {
		Help string `json:"help"`
		Err  string `json:"error,omitempty"`
	}{}
	if err := json.Unmarshal(data, &jsonable); err != nil {
		return err
	}
	if jsonable != nil {
		e.Help = jsonable.Help
		if jsonable.Err != "" {
			e.Err = errors.New(jsonable.Err)
		}
	}
	return nil
}

func CoverAllError(err error) *BaseError {
	return &BaseError{
		Err: err,
		Help: `Internal error: ` + err.Error() + `

We don't have a specific help message for the error above.

It would help us remedy this if you log an issue at

    https://github.com/weaveworks/flux/issues

saying what you were doing when you saw this, and quoting the message
at the top.
`,
	}
}

// A problem that is most likely caused by the user's configuration
// being incomplete or incorrect. For example, not having supplied a
// git repo.
type UserConfigProblem struct {
	*BaseError
}

// Something unexpected and bad happened and we're not sure why, but
// if you retry it may have come right again.
type ServerException struct {
	*BaseError
}

// The thing you asked for just doesn't exist. Sorry!
type Missing struct {
	*BaseError
}
