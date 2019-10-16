package git

import (
	"errors"
	"strings"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
)

var NoRepoError = &fluxerr.Error{
	Type: fluxerr.User,
	Err:  errors.New("no repo in user config"),
	Help: `No Git repository URL in your config

We need to clone a git repo to proceed, and you haven't supplied
one. Please upload a config file, including a git repository URL, as
described in

    https://docs.fluxcd.io/en/latest/references/fluxctl.html

`,
}

func CloningError(url string, actual error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  actual,
		Help: `Could not clone the upstream git repository

There was a problem cloning your git repository,

    ` + url + `

This may be because you have not supplied a valid deploy key, or
because the repository has been moved, deleted, or never existed.

Please check that there is a repository at the address above, and that
there is a deploy key with write permissions to the repository. In
GitHub, you can do this via the settings for the repository, and
cross-check with the fingerprint given by

    fluxctl identity

`,
	}
}

func ErrUpstreamNotWritable(url string, actual error) error {
	help := `Could not write to upstream repository

To keep track of synchronisation, the Flux daemon must be able to
write to the upstream git repository.
`
	if strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://") {
		help = help + `
Usually, git URLs starting with "http://" or "https://" will not work
well with flux, because they require the user to supply credentials
interactively. If possible, use an SSH URL (starting with "ssh://", or
of the form "user@host:path/to/repo").
`
	} else {
		help = help + `
This failure may be due to the SSH (deploy) key used by the daemon not
having write permission. You can see the key used, with

    fluxctl identity

In GitHub, please check via the repository settings that the deploy
key is "Read/write". You can cross-check the fingerprint with that
given by

    fluxctl identity --fingerprint

If the key is present but read-only, you will need to delete it and
create a new deploy key. To create a new one, use

    fluxctl identity --regenerate
`
	}

	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  actual,
		Help: help,
	}
}

func PushError(url string, actual error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  actual,
		Help: `Problem committing and pushing to git repository.

There was a problem with committing changes and pushing to the git
repository.

If this has worked before, it most likely means a fast-forward push
was not possible. It is safe to try again.

If it has not worked before, this probably means that the repository
exists but the SSH (deploy) key provided doesn't have write
permission.

In GitHub, please check via the repository settings that the deploy
key is "Read/write". You can cross-check the fingerprint with that
given by

    fluxctl identity

If the key is present but read-only, you will need to delete it and
create a new deploy key. To create a new one, use

    fluxctl identity --regenerate

The public key this outputs can then be given to GitHub; make sure you
check the box to allow write access unless you're using the
--git-readonly=true option.

`,
	}
}
