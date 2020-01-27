package git

import (
	"context"
	"os"
	"path/filepath"
)

type Export struct {
	dir string
}

func (e *Export) Dir() string {
	return e.dir
}

func (e *Export) Clean() error {
	if e.dir != "" {
		return os.RemoveAll(e.dir)
	}
	return nil
}

// Export creates a minimal clone of the repo, at the ref given.
func (r *Repo) Export(ctx context.Context, ref string) (*Export, error) {
	dir, err := r.workingClone(ctx, "")
	if err != nil {
		return nil, err
	}
	if err = checkout(ctx, dir, ref); err != nil {
		return nil, err
	}
	return &Export{dir}, nil
}

// SecretUnseal unseals git secrets in the clone.
func (e *Export) SecretUnseal(ctx context.Context) error {
	return secretUnseal(ctx, e.Dir())
}

// ChangedFiles does a git diff listing changed files
func (e *Export) ChangedFiles(ctx context.Context, sinceRef string, paths []string) ([]string, error) {
	list, err := changed(ctx, e.Dir(), sinceRef, paths)
	if err == nil {
		for i, file := range list {
			list[i] = filepath.Join(e.Dir(), file)
		}
	}
	return list, err
}
