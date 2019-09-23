package daemon

import (
	"fmt"
	"sync"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/resource"
)

type SyncErrors struct {
	errs map[resource.ID]error
	mu   sync.Mutex
}

func manifestLoadError(reason error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  reason,
		Help: `Unable to parse files as manifests

Flux was unable to parse the files in the git repo as manifests,
giving this error:

    ` + reason.Error() + `

Check that any files mentioned are well-formed, and resources are not
defined more than once. It's also worth reviewing

    https://docs.fluxcd.io/en/latest/requirements.html

to make sure you're not running into any corner cases.

If you think your files are all OK and you are still getting this
message, please log an issue at

    https://github.com/fluxcd/flux/issues

and include the problematic file, if possible.
`,
	}
}

func unknownJobError(id job.ID) error {
	return &fluxerr.Error{
		Type: fluxerr.Missing,
		Err:  fmt.Errorf("unknown job %q", string(id)),
		Help: `Job not found

This is often because the job did not result in committing changes,
and therefore had no lasting effect. A release dry-run is an example
of a job that does not result in a commit.

If you were expecting changes to be committed, this may mean that the
job failed, but its status was lost.

In both of the above cases it is OK to retry the operation that
resulted in this error.

If you get this error repeatedly, it's probably a bug. Please log an
issue describing what you were attempting, and posting logs from the
daemon if possible:

    https://github.com/fluxcd/flux/issues

`,
	}
}

func unsignedHeadRevisionError(latestValidRevision, headRevision string) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  fmt.Errorf("HEAD revision is unsigned"),
		Help: `HEAD is not a verified commit.

The branch HEAD in the git repo is not verified, and fluxd is unable to
make a change on top of it. The last verified commit was

    ` + latestValidRevision + `

HEAD is 

    ` + headRevision + `.
`,
	}
}
