package cache

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestWarming_ExpiryBuffer(t *testing.T) {
	testTime := time.Now()
	for _, x := range []struct {
		expiresIn, buffer time.Duration
		expectedResult    bool
	}{
		{time.Minute, time.Second, false},
		{time.Second, time.Minute, true},
	} {
		if withinExpiryBuffer(testTime.Add(x.expiresIn), x.buffer) != x.expectedResult {
			t.Fatalf("Should return %t", x.expectedResult)
		}
	}
}

func TestName(t *testing.T) {
	err := errors.Wrap(context.DeadlineExceeded, "getting remote manifest")
	t.Log(err.Error())
	err = errors.Cause(err)
	if err == context.DeadlineExceeded {
		t.Log("OK")
	} else {
		t.Log("Not OK")
	}
}
