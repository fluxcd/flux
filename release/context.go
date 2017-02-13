package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform/kubernetes"
)

type ReleaseContext struct {
	Instance   *instance.Instance
	WorkingDir string
}

func NewReleaseContext(inst *instance.Instance) *ReleaseContext {
	return &ReleaseContext{
		Instance: inst,
	}
}

// Preparation and cleanup

func (rc *ReleaseContext) CloneRepo() error {
	path, err := rc.Instance.ConfigRepo().Clone()
	if err != nil {
		return err
	}
	rc.WorkingDir = path
	return nil
}

func (rc *ReleaseContext) CommitAndPush(msg string) error {
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

// Selecting services

func LockedServices(config instance.Config) flux.ServiceIDSet {
	ids := []flux.ServiceID{}
	for id, s := range config.Services {
		if s.Locked {
			ids = append(ids, id)
		}
	}
	idSet := flux.ServiceIDSet{}
	idSet.Add(ids)
	return idSet
}

func (rc *ReleaseContext) SelectServices(only flux.ServiceIDSet, locked flux.ServiceIDSet, excluded flux.ServiceIDSet, results flux.ReleaseResult, logStatus statusFn) ([]*ServiceUpdate, error) {
	// Figure out all services that are defined in the repo and should
	// be selected for upgrading/applying.
	defined, err := rc.FindDefinedServices()
	if err != nil {
		return nil, err
	}

	updateMap := map[flux.ServiceID]*ServiceUpdate{}
	var ids []flux.ServiceID

	for _, s := range defined {
		if only == nil || only.Contains(s.ServiceID) {
			var result flux.ServiceResult
			switch {
			case excluded.Contains(s.ServiceID):
				logStatus("Skipping service %s as it is excluded", s.ServiceID)
				result = flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  "excluded",
				}
			case locked.Contains(s.ServiceID):
				logStatus("Skipping service %s as it is locked", s.ServiceID)
				result = flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  "locked",
				}
			default:
				result = flux.ServiceResult{
					Status: flux.ReleaseStatusPending,
				}
				updateMap[s.ServiceID] = s
				ids = append(ids, s.ServiceID)
			}
			results[s.ServiceID] = result
		}
	}

	// Correlate with services in running system.
	services, err := rc.Instance.GetServices(ids)
	if err != nil {
		return nil, err
	}

	var updates []*ServiceUpdate
	for _, service := range services {
		logStatus("Found service %s", service.ID)
		update := updateMap[service.ID]
		update.Service = service
		updates = append(updates, update)
		delete(updateMap, service.ID)
	}
	// Mark anything left over as skipped
	for id, _ := range updateMap {
		logStatus("Ignoring service %s as it is not in the running system", id)
		results[id] = flux.ServiceResult{
			Status: flux.ReleaseStatusIgnored,
			Error:  "not in running system",
		}
	}

	return updates, nil
}

func (rc *ReleaseContext) SelectExactServices(ss []flux.ServiceID, locked flux.ServiceIDSet, excluded flux.ServiceIDSet, results flux.ReleaseResult, logStatus statusFn) ([]*ServiceUpdate, error) {
	include := flux.ServiceIDSet{}
	include.Add(ss)
	return rc.SelectServices(include, locked, excluded, results, logStatus)
}

func (rc *ReleaseContext) FindDefinedServices() ([]*ServiceUpdate, error) {
	services, err := kubernetes.FindDefinedServices(rc.RepoPath())
	if err != nil {
		return nil, err
	}

	var defined []*ServiceUpdate
	for id, paths := range services {
		switch len(paths) {
		case 1:
			def, err := ioutil.ReadFile(paths[0])
			if err != nil {
				return nil, err
			}
			defined = append(defined, &ServiceUpdate{
				ServiceID:     id,
				ManifestPath:  paths[0],
				ManifestBytes: def,
			})
		default:
			return nil, fmt.Errorf("multiple resource files found for service %s: %s", id, strings.Join(paths, ", "))
		}
	}
	return defined, nil
}
