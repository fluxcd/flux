package release

import (
	"os"
	"path/filepath"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

type ReleaseContext struct {
	Instance       *instance.Instance
	WorkingDir     string
	KeyPath        string
	PodControllers map[flux.ServiceID][]byte
}

func NewReleaseContext(inst *instance.Instance) *ReleaseContext {
	return &ReleaseContext{
		Instance:       inst,
		PodControllers: map[flux.ServiceID][]byte{},
	}
}

func (rc *ReleaseContext) CloneConfig() error {
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

func (rc *ReleaseContext) ConfigPath() string {
	return filepath.Join(rc.WorkingDir, rc.Instance.ConfigRepo().Path)
}

func (rc *ReleaseContext) Clean() {
	if rc.WorkingDir != "" {
		os.RemoveAll(rc.WorkingDir)
	}
}
