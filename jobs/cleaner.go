package jobs

import (
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
)

type Cleaner struct {
	store  flux.JobStore
	logger log.Logger
}

func NewCleaner(store flux.JobStore, logger log.Logger) *Cleaner {
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
