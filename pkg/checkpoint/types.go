package checkpoint

import (
	"sync"
	"time"
)

type flag struct {
	Key   string
	Value string
}

type checkParams struct {
	Product       string
	Version       string
	Flags         map[string]string
	ExtraFlags    func() []flag
	Arch          string
	OS            string
	Signature     string
	SignatureFile string
	CacheFile     string
	CacheDuration time.Duration
	Force         bool
}

type checker struct {
	doneCh          chan struct{}
	nextCheckAt     time.Time
	nextCheckAtLock sync.RWMutex
}
