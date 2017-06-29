package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jonboulle/clockwork"
)

const (
	InitialBackoff = 500 * time.Millisecond
	MaxBackoff     = 10 * time.Second
)

var (
	ErrTimeout = errors.New("http request timeout")
)

type backoffRoundTripper struct {
	roundTripper               http.RoundTripper
	initialBackoff, maxBackoff time.Duration
	clock                      clockwork.Clock
}

// BackoffRoundTripper is a http.RoundTripper which adds a backoff for
// throttling to requests. To add a total request timeout, use Request.WithContext.
//
// r              -- upstream roundtripper
// initialBackoff -- initial length to backoff to when request request fails
// maxBackoff     -- maximum length to backoff to between request attempts
func BackoffRoundTripper(r http.RoundTripper, initialBackoff, maxBackoff time.Duration, clock clockwork.Clock) http.RoundTripper {
	return &backoffRoundTripper{
		roundTripper:   r,
		initialBackoff: initialBackoff,
		maxBackoff:     maxBackoff,
		clock:          clock,
	}
}

func (c *backoffRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	b := &backoff{
		initial: c.initialBackoff,
		max:     c.maxBackoff,
	}
	for {
		// Try the request
		resp, err := c.roundTripper.RoundTrip(r)
		switch {
		case err != nil && strings.Contains(err.Error(), "Too Many Requests (HAP429)."):
			// Catch the terrible dockerregistry error here. Eugh. :(
			fallthrough
		case resp != nil && resp.StatusCode == http.StatusTooManyRequests:
			fallthrough
		case resp != nil && resp.StatusCode >= 500:
			// Request rate-limited, backoff and retry.
			b.Failure()
			// Wait until the next time we are allowed to make a request
			c.clock.Sleep(b.Wait())
		default:
			return resp, err
		}
	}
}

// backoff calculates an exponential backoff. This is used to
// calculate wait times for future requests.
type backoff struct {
	initial time.Duration
	max     time.Duration

	current time.Duration
}

// Failure should be called each time a request fails.
func (b *backoff) Failure() {
	b.current *= 2
	if b.current == 0 {
		b.current = b.initial
	} else if b.current > b.max {
		b.current = b.max
	}
}

// Wait how long to sleep before *actually* starting the request.
func (b *backoff) Wait() time.Duration {
	return b.current
}
