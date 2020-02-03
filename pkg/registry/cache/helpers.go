package cache

/*
Just a set of utility to achieve DRY with backing storage.
 */

import (
	"encoding/binary"
	"time"
)

const (
	// The minimum expiry given to an entry.
	MinExpiry = time.Hour
)

func GracePeriodDeadline(refreshDeadline time.Time) time.Duration {
	expiry := refreshDeadline.Sub(time.Now()) * 2
	if expiry < MinExpiry {
		expiry = MinExpiry
	}
	return expiry
}

func EndianCompose(deadlineBytes, value []byte) []byte {
	return append(deadlineBytes, value...)
}

func EndianPut(refreshDeadline time.Time) (deadlineBytes []byte) {
	deadlineBytes = make([]byte, 4, 4)
	binary.BigEndian.PutUint32(deadlineBytes, uint32(refreshDeadline.Unix()))
	return
}

func EndianGet(cacheItem []byte) ([]byte, time.Time, error) {
	deadlineTime := binary.BigEndian.Uint32(cacheItem)
	return cacheItem[4:], time.Unix(int64(deadlineTime), 0), nil
}
