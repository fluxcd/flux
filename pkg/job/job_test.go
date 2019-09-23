package job

import (
	"sync"
	"testing"
)

func TestQueue(t *testing.T) {
	shutdown := make(chan struct{})
	wg := &sync.WaitGroup{}
	defer close(shutdown)
	q := NewQueue(shutdown, wg)
	if q.Len() != 0 {
		t.Errorf("Fresh queue has length %d (!= 0)", q.Len())
	}

	select {
	case <-q.Ready():
		t.Error("Value from q.Ready before any values enqueued")
	default:
	}

	// When this proceeds, the value will be in the queue
	q.Enqueue(&Job{"job 1", nil})
	q.Sync()
	if q.Len() != 1 {
		t.Errorf("Queue has length %d (!= 1) after enqueuing one item (and sync)", q.Len())
	}

	// This should proceed eventually
	j := <-q.Ready()
	if j.ID != "job 1" {
		t.Errorf("Dequeued odd job: %#v", j)
	}
	q.Sync()
	if q.Len() != 0 {
		t.Errorf("Queue has length %d (!= 0) after dequeuing only item (and sync)", q.Len())
	}

	// This should not proceed, because the queue is empty
	select {
	case j = <-q.Ready():
		t.Errorf("Dequeued from empty queue: %#v", j)
	default:
	}
}
