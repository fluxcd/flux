package release

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

type ReleaseContext struct {
	Instance           *instance.Instance
	WorkingDir         string
	KeyPath            string
	ServiceDefinitions map[flux.ServiceID][]string
	Images             instance.ImageMap
}

func NewReleaseContext(inst *instance.Instance) *ReleaseContext {
	return &ReleaseContext{
		Instance:           inst,
		ServiceDefinitions: map[flux.ServiceID][]string{},
		Images:             instance.ImageMap{},
	}
}

func (rc *ReleaseContext) RepoURL() string {
	return rc.Instance.ConfigRepo().URL
}

func (rc *ReleaseContext) CloneRepo() error {
	path, keyfile, err := rc.Instance.ConfigRepo().Clone()
	if err != nil {
		return err
	}
	rc.WorkingDir = path
	rc.KeyPath = keyfile
	return nil
}

func (rc *ReleaseContext) CommitAndPush(msg string) (string, error) {
	return rc.Instance.ConfigRepo().CommitAndPush(rc.WorkingDir, rc.KeyPath, msg)
}

func (rc *ReleaseContext) RepoPath() string {
	return filepath.Join(rc.WorkingDir, rc.Instance.ConfigRepo().Path)
}

func (rc *ReleaseContext) Clean() {
	if rc.WorkingDir != "" {
		os.RemoveAll(rc.WorkingDir)
	}
}
