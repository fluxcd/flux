package release

import (
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/fluxy"
)

type Cleaner struct {
	store  flux.ReleaseJobStore
	logger log.Logger
}

func NewCleaner(store flux.ReleaseJobStore, logger log.Logger) *Cleaner {
	return &Cleaner{
		store:  store,
		logger: logger,
	}
}

func (c *Cleaner) Clean(tick <-chan time.Time) {
	for range tick {
		if err := c.store.GC(); err != nil {
			c.logger.Log("err", err)
		}
	}
}
