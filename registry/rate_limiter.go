package registry

import (
	"context"
	"golang.org/x/time/rate"
	"net/http"
	"time"
)

var limiters = make(map[string]http.RoundTripper)

type RateLimiterConfig struct {
	RPS   int           // Rate per second per host
	Burst int           // Burst count per host
	Wait  time.Duration // Maximum wait time for a request
}

func RateLimitedRoundTripper(rt http.RoundTripper, config RateLimiterConfig, host string) http.RoundTripper {
	if _, ok := limiters[host]; !ok {
		rl := rate.NewLimiter(rate.Limit(config.RPS), config.Burst)
		limiters[host] = &RoundTripRateLimiter{
			Wait:      config.Wait,
			RL:        rl,
			Transport: rt,
		}
	}
	return limiters[host]
}

type RoundTripRateLimiter struct {
	Wait      time.Duration // Maximum wait time for a request
	RL        *rate.Limiter
	Transport http.RoundTripper
}

func (rl *RoundTripRateLimiter) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(r.Context(), rl.Wait)
	defer cancel() // always cancel the context!

	// Wait errors out if the request cannot be processed within
	// the deadline. This is preemptive, instead of waiting the
	// entire duration.
	rl.RL.Allow()
	if err := rl.RL.Wait(ctx); err != nil {
		return nil, err
	}
	return rl.Transport.RoundTrip(r)
}
