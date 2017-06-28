package registry

import (
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
	"net/http"
)

var limiters = make(map[string]*rate.Limiter)

type RateLimiterConfig struct {
	RPS   int // Rate per second per host
	Burst int // Burst count per host
}

func RateLimitedRoundTripper(rt http.RoundTripper, config RateLimiterConfig, host string) http.RoundTripper {
	if _, ok := limiters[host]; !ok {
		rl := rate.NewLimiter(rate.Limit(config.RPS), config.Burst)
		limiters[host] = rl
	}
	return &RoundTripRateLimiter{
		RL:        limiters[host],
		Transport: rt,
	}
}

type RoundTripRateLimiter struct {
	RL        *rate.Limiter
	Transport http.RoundTripper
}

func (rl *RoundTripRateLimiter) RoundTrip(r *http.Request) (*http.Response, error) {
	// Wait errors out if the request cannot be processed within
	// the deadline. This is preemptive, instead of waiting the
	// entire duration.
	if err := rl.RL.Wait(r.Context()); err != nil {
		return nil, errors.Wrap(err, "rate limited")
	}
	return rl.Transport.RoundTrip(r)
}
