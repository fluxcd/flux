package middleware

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

const (
	minLimit  = 0.1
	backOffBy = 2.0
	recoverBy = 1.5
)

// RateLimiters keeps track of per-host rate limiting for an arbitrary
// set of hosts.

// Use `*RateLimiter.RoundTripper(host)` to obtain a rate limited HTTP
// transport for an operation. The RoundTripper will react to a `HTTP
// 429 Too many requests` response by reducing the limit for that
// host. It will only do so once, so that concurrent requests don't
// *also* reduce the limit.
//
// Call `*RateLimiter.Recover(host)` when an operation has succeeded
// without incident, which will increase the rate limit modestly back
// towards the given ideal.
type RateLimiters struct {
	RPS     float64
	Burst   int
	Logger  log.Logger
	perHost map[string]*rate.Limiter
	mu      sync.Mutex
}

func (limiters *RateLimiters) clip(limit float64) float64 {
	if limit < minLimit {
		return minLimit
	}
	if limit > limiters.RPS {
		return limiters.RPS
	}
	return limit
}

// backOff can be called to explicitly reduce the limit for a
// particular host. Usually this isn't necessary since a RoundTripper
// obtained for a host will respond to `HTTP 429` by doing this for
// you.
func (limiters *RateLimiters) backOff(host string) {
	limiters.mu.Lock()
	defer limiters.mu.Unlock()

	var limiter *rate.Limiter
	if limiters.perHost == nil {
		limiters.perHost = map[string]*rate.Limiter{}
	}
	if rl, ok := limiters.perHost[host]; ok {
		limiter = rl
	} else {
		limiter = rate.NewLimiter(rate.Limit(limiters.RPS), limiters.Burst)
		limiters.perHost[host] = limiter
	}

	oldLimit := float64(limiter.Limit())
	newLimit := limiters.clip(oldLimit / backOffBy)
	if oldLimit != newLimit && limiters.Logger != nil {
		limiters.Logger.Log("info", "reducing rate limit", "host", host, "limit", strconv.FormatFloat(newLimit, 'f', 2, 64))
	}
	limiter.SetLimit(rate.Limit(newLimit))
}

// Recover should be called when a use of a RoundTripper has
// succeeded, to bump the limit back up again.
func (limiters *RateLimiters) Recover(host string) {
	limiters.mu.Lock()
	defer limiters.mu.Unlock()
	if limiters.perHost == nil {
		return
	}
	if limiter, ok := limiters.perHost[host]; ok {
		oldLimit := float64(limiter.Limit())
		newLimit := limiters.clip(oldLimit * recoverBy)
		if newLimit != oldLimit && limiters.Logger != nil {
			limiters.Logger.Log("info", "increasing rate limit", "host", host, "limit", strconv.FormatFloat(newLimit, 'f', 2, 64))
		}
		limiter.SetLimit(rate.Limit(newLimit))
	}
}

// Limit returns a RoundTripper for a particular host. We expect to do
// a number of requests to a particular host at a time.
func (limiters *RateLimiters) RoundTripper(rt http.RoundTripper, host string) http.RoundTripper {
	limiters.mu.Lock()
	defer limiters.mu.Unlock()

	if limiters.perHost == nil {
		limiters.perHost = map[string]*rate.Limiter{}
	}
	if _, ok := limiters.perHost[host]; !ok {
		rl := rate.NewLimiter(rate.Limit(limiters.RPS), limiters.Burst)
		limiters.perHost[host] = rl
	}
	var reduceOnce sync.Once
	return &roundTripRateLimiter{
		rl: limiters.perHost[host],
		tx: rt,
		slowDown: func() {
			reduceOnce.Do(func() { limiters.backOff(host) })
		},
	}
}

type roundTripRateLimiter struct {
	rl       *rate.Limiter
	tx       http.RoundTripper
	slowDown func()
}

func (t *roundTripRateLimiter) RoundTrip(r *http.Request) (*http.Response, error) {
	// Wait errors out if the request cannot be processed within
	// the deadline. This is pre-emptive, instead of waiting the
	// entire duration.
	if err := t.rl.Wait(r.Context()); err != nil {
		return nil, errors.Wrap(err, "rate limited")
	}
	resp, err := t.tx.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		t.slowDown()
	}
	return resp, err
}
