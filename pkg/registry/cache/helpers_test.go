package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGracePeriodDeadline(t *testing.T) {
	t.Run("yesterday", func(t *testing.T) {
		r := GracePeriodDeadline(time.Now().Add(-24*time.Hour))
		assert.Equal(t, r, MinExpiry)
	})
	t.Run("tomorrow", func(t *testing.T) {
		r := GracePeriodDeadline(time.Now().Add(24*time.Hour))
		assert.NotEqual(t, r, MinExpiry)
	})
}

func TestEndianCompose(t *testing.T) {
	type tc struct {
		db []byte
		v  []byte

		e  []byte
	}
	for _, v := range []tc {
		{db: []byte("^"), v: []byte("qwerty"), e: []byte("^qwerty")},
		{db: []byte("!2#4"), v: []byte("foobarbizzbuzz"), e: []byte("!2#4foobarbizzbuzz")},
		{db: []byte("!@#$%^&*()"), v: []byte("42istheanswer"), e: []byte("!@#$%^&*()42istheanswer")},
	} {
		assert.Equal(t, v.e, EndianCompose(v.db, v.v))
	}
}

func TestEndianFlow(t *testing.T) {
	e := time.Now().Add(time.Hour).Round(time.Second)

	p := EndianPut(e)
	_, a, err := EndianGet(p)

	assert.Nil(t, err)
	assert.Equal(t, e, a)
}
