package job

import (
	"sync"

	"github.com/go-kit/kit/log"

	"github.com/fluxcd/flux/pkg/update"
)

type ID string

type JobFunc func(log.Logger) error

type Job struct {
	ID ID
	Do JobFunc
}

type StatusString string

const (
	StatusQueued    StatusString = "queued"
	StatusRunning   StatusString = "running"
	StatusFailed    StatusString = "failed"
	StatusSucceeded StatusString = "succeeded"
)

// Result looks like CommitEventMetadata, because that's what we
// used to send. But in the interest of breaking cycles before
// they happen, it's (almost) duplicated here.
type Result struct {
	Revision string        `json:"revision,omitempty"`
	Spec     *update.Spec  `json:"spec,omitempty"`
	Result   update.Result `json:"result,omitempty"`
}

// Status holds the possible states of a job; either,
//  1. queued or otherwise pending
//  2. succeeded with a job-specific result
//  3. failed, resulting in an error and possibly a job-specific result
type Status struct {
	Result       Result
	Err          string
	StatusString StatusString
}

func (s Status) Error() string {
	return s.Err
}

// Queue is an unbounded queue of jobs; enqueuing a job will always
// proceed, while dequeuing is done by receiving from a channel. It is
// also possible to iterate over the current list of jobs.
type Queue struct {
	ready       chan *Job
	incoming    chan *Job
	waiting     []*Job
	waitingLock sync.Mutex
	sync        chan struct{}
}

func NewQueue(stop <-chan struct{}, wg *sync.WaitGroup) *Queue {
	q := &Queue{
		ready:    make(chan *Job),
		incoming: make(chan *Job),
		waiting:  make([]*Job, 0),
		sync:     make(chan struct{}),
	}
	wg.Add(1)
	go q.loop(stop, wg)
	return q
}

// This is not guaranteed to be up-to-date; i.e., it is possible to
// receive from `q.Ready()` or enqueue an item, then see the same
// length as before, temporarily.
func (q *Queue) Len() int {
	q.waitingLock.Lock()
	defer q.waitingLock.Unlock()
	return len(q.waiting)
}

// Enqueue puts a job onto the queue. It will block until the queue's
// loop can accept the job; but this does _not_ depend on a job being
// dequeued and will always proceed eventually.
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

// Block until any previous operations have completed. Note that this
// is only meaningful if you are using the queue from a single other
// goroutine; i.e., it makes sense to do, say,
//
//    q.Enqueue(j)
//    q.Sync()
//    fmt.Printf("Queue length is %d\n", q.Len())
//
// but only because those statements are sequential in a single
// thread. So this is really only useful for testing.
func (q *Queue) Sync() {
	q.sync <- struct{}{}
}

func (q *Queue) loop(stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		var out chan *Job = nil
		if len(q.waiting) > 0 {
			out = q.ready
		}

		select {
		case <-stop:
			return
		case <-q.sync:
			continue
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

// nextOrNil returns the head of the queue, or nil if
// the queue is empty.
func (q *Queue) nextOrNil() *Job {
	q.waitingLock.Lock()
	defer q.waitingLock.Unlock()
	if len(q.waiting) > 0 {
		return q.waiting[0]
	}
	return nil
}
