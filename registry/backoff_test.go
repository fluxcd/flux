package registry

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func TestBackoff(t *testing.T) {
	initial := 1 * time.Second
	max := 10 * time.Second
	b := &backoff{
		initial: initial,
		max:     max,
	}
	// It should start with no wait
	if b.Wait() != 0 {
		t.Errorf("Expected backoff to start with no wait, got %v", b.Wait())
	}

	for i, expected := range []time.Duration{
		initial,         // 1 failures, initial backoff
		2 * time.Second, // 2 failures
		4 * time.Second, // 3 failures
		8 * time.Second, // 4 failures
		max,             // 5 failures, max backoff
		max,             // 6 failures, max backoff
	} {
		b.Failure()
		if b.Wait() != expected {
			t.Errorf("Expected backoff after %d failures to be %v, got %v", i+1, expected, b.Wait())
		}
	}
}

func TestBackoffRoundTripper(t *testing.T) {
	clock := clockwork.NewFakeClock()
	req, _ := http.NewRequest("GET", "http://example.com/foo", nil)

	// it should immediately return the response when successful
	{
		calls := []time.Time{}
		rt := roundtripperFunc(func(r *http.Request) (*http.Response, error) {
			calls = append(calls, clock.Now())
			return &http.Response{StatusCode: http.StatusOK}, nil
		})
		resp, err := BackoffRoundTripper(rt, 1*time.Second, 10*time.Second, clock).RoundTrip(req)
		if err != nil {
			t.Error(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected successful response, got %v", resp)
		}
		if len(calls) != 1 {
			t.Errorf("Expected 1 successful call, got %v", calls)
		}
	}

	for _, failures := range []struct {
		*http.Response
		error
	}{
		// it should catch the HAP error from dockerregistry
		{nil, errors.New("Too Many Requests (HAP429).")},
		// it should catch http.StatusTooManyRequests
		{&http.Response{StatusCode: http.StatusTooManyRequests}, nil},
		// it should catch http.StatusInternalServerError
		{&http.Response{StatusCode: http.StatusInternalServerError}, nil},
	} {
		calls := []time.Time{}
		rt := roundtripperFunc(func(r *http.Request) (*http.Response, error) {
			calls = append(calls, clock.Now())
			if len(calls) <= 1 {
				return failures.Response, failures.error
			}
			return &http.Response{StatusCode: http.StatusOK}, nil
		})
		done := make(chan struct{})
		go func() {
			resp, err := BackoffRoundTripper(rt, 1*time.Second, 10*time.Second, clock).RoundTrip(req)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if resp != nil && resp.StatusCode != http.StatusOK {
				t.Errorf("Expected successful response, got %v", resp)
			}
			if len(calls) != 2 {
				t.Errorf("Expected 2 calls, got %v", calls)
			}
			close(done)
		}()
		clock.BlockUntil(1)
		clock.Advance(1001 * time.Millisecond)
		<-done
	}
}

type roundtripperFunc func(*http.Request) (*http.Response, error)

func (f roundtripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
