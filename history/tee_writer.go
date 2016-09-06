package history

import (
	"fmt"
	"strings"
)

func TeeWriter(w ...EventWriter) EventWriter {
	return teeWriter(w)
}

type teeWriter []EventWriter

func (w teeWriter) LogEvent(namespace, service, msg string) error {
	// Attempt to write to all. All errors are captured.
	var errs []string
	for _, w0 := range w {
		if err := w0.LogEvent(namespace, service, msg); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}
