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

// Clone clones the repo to a temporary path. If Clone returns a nil error, the
// caller is responsible for os.RemoveAll-ing the path.
func (r Repo) Clone() (path string, err error) {
	return clone(r.Key, r.URL)
}
