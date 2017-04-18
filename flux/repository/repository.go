package repository

// Repository collects behaviors related to e.g. git repos.
type Repository interface {
	// Clone clones the given repository to a temp directory.
	// The returned key file is necessary to commit and push.
	Clone(uri string) (repoPath, keyFile string, err error)

	// CommitAndPush commits and pushes the repo.
	// If there have been no changes, ErrNoChanges is returned.
	CommitAndPush(repoPath, keyFile, commitMessage string) error
}
