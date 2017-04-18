package release

// Worker consumes jobs from the job store.
type Worker struct {
	j JobPopper
}
