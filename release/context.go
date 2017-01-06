package release

import (
	"os"
	"path/filepath"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

type ReleaseContext struct {
	Instance           *instance.Instance
	WorkingDir         string
	ServiceDefinitions map[flux.ServiceID]map[string][]byte
	Images             instance.ImageMap
	ServiceImages      map[flux.ServiceID][]flux.ImageID
	UpdatedDefinitions map[flux.ServiceID]map[string][]byte
}

func NewReleaseContext(inst *instance.Instance) *ReleaseContext {
	return &ReleaseContext{
		Instance:           inst,
		ServiceDefinitions: map[flux.ServiceID]map[string][]byte{},
		Images:             instance.ImageMap{},
		ServiceImages:      map[flux.ServiceID][]flux.ImageID{},
		UpdatedDefinitions: map[flux.ServiceID]map[string][]byte{},
	}
}

func (rc *ReleaseContext) RepoURL() string {
	return rc.Instance.ConfigRepo().URL
}

func (rc *ReleaseContext) CloneRepo() error {
	path, err := rc.Instance.ConfigRepo().Clone()
	if err != nil {
		return err
	}
	rc.WorkingDir = path
	return nil
}

func (rc *ReleaseContext) CommitAndPush(msg string) (string, error) {
	return rc.Instance.ConfigRepo().CommitAndPush(rc.WorkingDir, msg)
}

func (rc *ReleaseContext) RepoPath() string {
	return filepath.Join(rc.WorkingDir, rc.Instance.ConfigRepo().Path)
}

func (rc *ReleaseContext) Clean() {
	if rc.WorkingDir != "" {
		os.RemoveAll(rc.WorkingDir)
	}
}
