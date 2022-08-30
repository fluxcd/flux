package checkpoint

import (
	mrand "math/rand"
	"os"
	"time"
)

func isCheckDisabled() bool {
	return os.Getenv("CHECKPOINT_DISABLE") != ""
}

func randomStagger(interval time.Duration) time.Duration {
	stagger := time.Duration(mrand.Int63()) % (interval / 2)
	return 3*(interval/4) + stagger
}
