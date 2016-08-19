package git

// Repo represents a remote git repo
type Repo struct {
	// The URL to the config repo that holds the resource definition files. For
	// example, "https://github.com/myorg/conf.git", "git@foo.com:myorg/conf".
	URL string

	// The file containing the private key with permissions to clone and push to
	// the config repo.
	Key string

	// The path within the config repo where files are stored.
	Path string
}

func (r Repo) Clone() (path string, err error) {
	return clone(r.Key, r.URL)
}

func (r Repo) CommitAndPush(path, commitMessage string) error {
	if err := commit(path, commitMessage); err != nil {
		return err
	}
	return push(r.Key, path)
}
