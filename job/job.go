package job

import (
	"sync"
)

type ID string

type Job struct {
	ID ID
	Do func() error
}

type StatusString string

const (
	StatusQueued    StatusString = "queued"
	StatusRunning   StatusString = "running"
	StatusFailed    StatusString = "failed"
	StatusSucceeded StatusString = "succeeded"
)

// Status holds the possible states of a job; either,
//  1. queued or otherwise pending
//  2. succeeded with a job-specific result
//  3. failed, resulting in an error and possibly a job-specific result
type Status struct {
	Result       interface{}
	Error        error
	StatusString StatusString
}

// Queue is an unbounded queue of jobs; enqueuing a job will always
// proceed, while dequeuing is done by receiving from a channel. It is
// also possible to iterate over the current list of jobs.
type Queue struct {
	ready       chan *Job
	incoming    chan *Job
	waiting     []*Job
	waitingLock sync.Mutex
}

func NewQueue() *Queue {
	return &Queue{
		ready:    make(chan *Job),
		incoming: make(chan *Job),
		waiting:  make([]*Job, 0),
	}
}

func (q *Queue) Len() int {
	q.waitingLock.Lock()
	defer q.waitingLock.Unlock()
	return len(q.waiting)
}

// Enqueue puts a job onto the queue. It will block until the queue's
// loop can accept the job; this does _not_ depend on a job being
// dequeued.
func (q *Queue) Enqueue(j *Job) {
	q.incoming <- j
}

// Ready returns a channel that can be used to dequeue items. Note
// that dequeuing is not atomic: you may still see the
// dequeued item with ForEach, for a time.
func (q *Queue) Ready() <-chan *Job {
	return q.ready
}

func (q *Queue) ForEach(fn func(int, *Job) bool) {
	q.waitingLock.Lock()
	jobs := q.waiting
	q.waitingLock.Unlock()
	for i, job := range jobs {
		if !fn(i, job) {
			return
		}
	}
}

func (q *Queue) Loop(stop chan struct{}) {
	defer q.stop()
	for {
		var out chan *Job = nil
		if len(q.waiting) > 0 {
			out = q.ready
		}

		select {
		case <-stop:
			return
		case in := <-q.incoming:
			q.waitingLock.Lock()
			q.waiting = append(q.waiting, in)
			q.waitingLock.Unlock()
		case out <- q.nextOrNil(): // cannot proceed if out is nil
			q.waitingLock.Lock()
			q.waiting = q.waiting[1:]
			q.waitingLock.Unlock()
		}
	}
}

func (q *Queue) stop() {
	// unblock anyone waiting on a value
	close(q.ready)
	// unblock anyone waiting on incoming (possibly discarding a
	// value)
	select {
	case <-q.incoming:
	default:
	}
}

// nextOrNil returns the next job that will be made ready, or nil if
// the queue is empty.
func (q *Queue) nextOrNil() *Job {
	q.waitingLock.Lock()
	defer q.waitingLock.Unlock()
	if len(q.waiting) > 0 {
		return q.waiting[0]
	}
	return nil
}
