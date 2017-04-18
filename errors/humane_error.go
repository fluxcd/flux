package errors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
)

type HumaneError struct {
	Cause   error       `json:"cause"`   // Another error that caused this one
	Code    int         `json:"code"`    // Generally (but not necessarily) a HTTP status code.
	Help    string      `json:"help"`    // A user facing message
	Details interface{} `json:"details"` // Encode developer messages in here
	Stack   Stack       `json:"stack"`   // The stack trace for this call
}

// New returns a smashing humane error
func New(code int, usrMsg string, details interface{}) *HumaneError {
	return Wrap(nil, code, usrMsg, details)
}

// Wrap follows pkg.errors convention and nests an error within a new error
func Wrap(err error, code int, usrMsg string, details interface{}) *HumaneError {
	return &HumaneError{
		Cause:   err,
		Code:    code,
		Help:    usrMsg,
		Details: details,
		Stack:   stackTrace(),
	}
}

func CoverallError(err error) *HumaneError {
	return Wrap(err, http.StatusInternalServerError,
		`An error occured for which we don't have a specific message.

	 If you see this, it means we need to come up with a better message! It
	 would help us if you log an issue at
	 https://github.com/weaveworks/flux/issues saying what you were doing
	 when you saw this, and quoting the following:

	     `, nil)
}

// Error satisfies the error interface.
func (e *HumaneError) Error() string {
	return fmt.Sprintf("%s Caused by: %q", e.Help, e.RootCause().Error())
}

func (e *HumaneError) RootCause() error {
	if e.Cause == nil {
		return e
	}
	next, isHumane := e.Cause.(*HumaneError)
	if isHumane {
		return next.RootCause()
	}
	return e.Cause
}

// For use in logs
type LogError struct {
	HumaneError
}

func (e *LogError) Error() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("Error %d: %s\n %#v%s", e.Code, e.Help, e.Details, e.Stack.String()))
	if e.Cause != nil {
		buffer.WriteString("\n\nCaused by:\n")
		buffer.WriteString(e.Cause.Error())
	}
	return buffer.String()
}

// For use in HTTP writers
type JSONError struct {
	HumaneError
}

func (e *JSONError) Error() string {
	b, _ := json.MarshalIndent(e, "", "\t")
	return string(b)
}

// Stack represents the full stack trace
type Stack []StackFrame

// StackFrame represents a single frame of a stack trace
type StackFrame struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

// String converts a stack trace to a string
func (s Stack) String() string {
	var buffer bytes.Buffer
	for _, v := range s {
		buffer.WriteString(fmt.Sprintf("\n- %s:%d %s", v.File, v.Line, v.Function))
	}
	return buffer.String()
}

// stackTrace gets the current stack trace
func stackTrace() Stack {
	stack := make([]StackFrame, 0)
	for i := 2; ; i++ {
		pc, fn, line, ok := runtime.Caller(i)
		if !ok {
			// no more frames - we're done
			break
		}
		_, fn = filepath.Split(fn)

		f := StackFrame{File: fn, Line: line, Function: funcName(pc)}
		stack = append(stack, f)
	}
	return stack
}

// funcName gets the name of the function at pointer or "??" if one can't be found
func funcName(pc uintptr) string {
	if f := runtime.FuncForPC(pc); f != nil {
		return f.Name()
	}
	return "??"
}
