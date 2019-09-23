package httperror

import (
	"fmt"
	"net/http"
)

// When an API call fails, we may want to distinguish among the causes
// by status code. This type can be used as the base error when we get
// a non-"HTTP 20x" response, retrievable with errors.Cause(err).
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (err *APIError) Error() string {
	return fmt.Sprintf("%s (%s)", err.Status, err.Body)
}

// Does this error mean the API service is unavailable?
func (err *APIError) IsUnavailable() bool {
	switch err.StatusCode {
	case 502, 503, 504:
		return true
	}
	return false
}

// Is this API call missing? This usually indicates that there is a
// version mismatch between the client and the service.
func (err *APIError) IsMissing() bool {
	return err.StatusCode == http.StatusNotFound
}
