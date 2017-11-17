package middleware

import (
	"context"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

type RateLimiters struct {
	RPS, Burst int
	perHost    map[string]*rate.Limiter
	mx         sync.Mutex
}

// Limit returns a RoundTripper for a particular host. We expect to do
// a number of requests to a particular host at a time.
func (limiters *RateLimiters) RoundTripper(rt http.RoundTripper, host string) http.RoundTripper {
	limiters.mx.Lock()
	defer limiters.mx.Unlock()

	if limiters.perHost == nil {
		limiters.perHost = map[string]*rate.Limiter{}
	}
	if _, ok := limiters.perHost[host]; !ok {
		rl := rate.NewLimiter(rate.Limit(limiters.RPS), limiters.Burst)
		limiters.perHost[host] = rl
	}
	return &RoundTripRateLimiter{
		rl: limiters.perHost[host],
		tx: rt,
	}
}

type RoundTripRateLimiter struct {
	rl *rate.Limiter
	tx http.RoundTripper
}

func (t *RoundTripRateLimiter) RoundTrip(r *http.Request) (*http.Response, error) {
	// Wait errors out if the request cannot be processed within
	// the deadline. This is preemptive, instead of waiting the
	// entire duration.
	if err := t.rl.Wait(r.Context()); err != nil {
		return nil, errors.Wrap(err, "rate limited")
	}
	return t.tx.RoundTrip(r)
}

type ContextRoundTripper struct {
	Transport http.RoundTripper
	Ctx       context.Context
}

func (rt *ContextRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.Transport.RoundTrip(r.WithContext(rt.Ctx))
}
