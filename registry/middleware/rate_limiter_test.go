package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const requestTimeout = 10 * time.Second

// We shouldn't share roundtrippers in the ratelimiter because the context will be stale
func TestRateLimiter_WithContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	rateLimit := 100

	// A context we'll use to cancel
	ctx, cancel := context.WithCancel(context.Background())
	rt := &ContextRoundTripper{Transport: http.DefaultTransport, Ctx: ctx}
	rl := RateLimitedRoundTripper(rt, RateLimiterConfig{
		RPS:   rateLimit,
		Burst: 1,
	}, ts.URL)

	client := &http.Client{
		Transport: rl,
		Timeout:   requestTimeout,
	}
	_, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	cancel() // Perform the cancel. If the RL is sharing contexts, then if we use it again it will fail.

	// Now do that again, it should use a new context
	// A context we'll use to cancel requests on error
	ctx, cancel = context.WithCancel(context.Background())
	rt = &ContextRoundTripper{Transport: http.DefaultTransport, Ctx: ctx}
	rl = RateLimitedRoundTripper(rt, RateLimiterConfig{
		RPS:   rateLimit,
		Burst: 1,
	}, ts.URL)
	client = &http.Client{
		Transport: rl,
		Timeout:   requestTimeout,
	}
	_, err = client.Get(ts.URL)
	if err != nil {
		t.Fatal(err) // It will fail here if it is sharing contexts
	}
	cancel()
}
