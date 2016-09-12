package cache

import (
	"testing"
	"time"
)

func TestWaiters(t *testing.T) {
	var (
		key   = "foo"
		value = 123
		count int
		load  = func() (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			count++ // should only happen once
			return value, nil
		}
	)

	var (
		c    = New()
		n    = 100
		valc = make(chan int)
		errc = make(chan error)
	)

	for i := 0; i < n; i++ {
		go func() {
			if val, err := c.Get(key, load); err != nil {
				errc <- err
			} else {
				valc <- val.(int)
			}
		}()
	}

	for i := 0; i < n; i++ {
		select {
		case val := <-valc:
			if val != value {
				t.Fatalf("want %d, have %d", value, val)
			}
		case err := <-errc:
			t.Fatal(err) // a single error is fatal
		case <-time.After(time.Second):
			t.Fatal("timeout") // bonk
		}
	}

	if want, have := 1, count; want != have {
		t.Errorf("want %d, have %d", want, have)
	}
}
