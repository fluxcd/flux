package automator

import (
	"strings"
	"testing"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
)

type mockInstanceDB map[flux.InstanceID]instance.Config

func (db mockInstanceDB) UpdateConfig(inst flux.InstanceID, update instance.UpdateFunc) error {
	newConfig, err := update(db[inst])
	if err == nil {
		db[inst] = newConfig
	}
	return err
}

func (db mockInstanceDB) GetConfig(inst flux.InstanceID) (instance.Config, error) {
	return db[inst], nil
}

func (db mockInstanceDB) All() ([]instance.NamedConfig, error) {
	var result []instance.NamedConfig
	for id, config := range db {
		result = append(result, instance.NamedConfig{
			ID:     flux.InstanceID(id),
			Config: config,
		})
	}
	return result, nil
}

type mockJobStore struct {
	getJob                   func(flux.InstanceID, jobs.JobID) (jobs.Job, error)
	nextJob                  func([]string) (jobs.Job, error)
	putJob                   func(flux.InstanceID, jobs.Job) (jobs.JobID, error)
	putJobIgnoringDuplicates func(flux.InstanceID, jobs.Job) (jobs.JobID, error)
	gc                       func() error
	updateJob                func(jobs.Job) error
	heartbeat                func(jobs.JobID) error
}

func (m *mockJobStore) GetJob(i flux.InstanceID, j jobs.JobID) (jobs.Job, error) {
	return m.getJob(i, j)
}

func (m *mockJobStore) NextJob(queues []string) (jobs.Job, error) {
	return m.nextJob(queues)
}

func (m *mockJobStore) PutJob(i flux.InstanceID, j jobs.Job) (jobs.JobID, error) {
	return m.PutJob(i, j)
}

func (m *mockJobStore) PutJobIgnoringDuplicates(i flux.InstanceID, j jobs.Job) (jobs.JobID, error) {
	return m.PutJobIgnoringDuplicates(i, j)
}

func (m *mockJobStore) GC() error                    { return m.gc() }
func (m *mockJobStore) UpdateJob(j jobs.Job) error   { return m.updateJob(j) }
func (m *mockJobStore) Heartbeat(j jobs.JobID) error { return m.heartbeat(j) }

var nullLogger = log.NewNopLogger()

func TestHandleAutomatedInstanceJob(t *testing.T) {
	instID := flux.InstanceID("instance1")
	serviceID := flux.ServiceID("ns1/service1")
	instanceDB := mockInstanceDB{
		instID: instance.Config{},
	}
	instancer := instance.StandaloneInstancer{
		Instance:  instID,
		Connecter: platform.NewStandaloneMessageBus(platform.NewBusMetrics()),
		Registry:  nil, // *registry.Client
		Config:    instanceDB,
		GitRepo:   git.Repo{},
	}
	a, err := New(Config{
		Jobs:       &mockJobStore{},
		InstanceDB: instanceDB,
		Instancer:  instancer,
		Logger:     nullLogger,
	})
	if err != nil {
		t.Fatal(err)
	}

	job := &jobs.Job{
		Instance: instID,
		// Key stops us getting two jobs for the same service
		Key: strings.Join([]string{
			jobs.AutomatedInstanceJob,
			string(instID),
		}, "|"),
		Method:   jobs.AutomatedInstanceJob,
		Priority: jobs.PriorityBackground,
		Params: jobs.AutomatedInstanceJobParams{
			InstanceID: instID,
		},
	}

	// When there are not automated services
	{
		followUps, err := a.handleAutomatedInstanceJob(nullLogger, job)
		if err != nil {
			t.Fatal(err)
		}
		// - It should not reschedule itself
		// - There should be no releases
		if len(followUps) != 0 {
			t.Error("Unexpected follow-up jobs when no automated services: %q", followUps)
		}
	}

	// When there are automated (but locked) services
	{
		instanceDB[instID] = instance.Config{
			Services: map[flux.ServiceID]instance.ServiceConfig{
				serviceID: instance.ServiceConfig{
					Automated: true,
					Locked:    true,
				},
			},
		}
		followUps, err := a.handleAutomatedInstanceJob(nullLogger, job)
		if err != nil {
			t.Fatal(err)
		}
		// - It should not reschedule itself
		// - There should be no releases
		if len(followUps) != 0 {
			t.Error("Unexpected follow-up jobs when automated (but locked) services: %q", followUps)
		}
	}

	// When there are automated services (with no new images available)
	{
		instanceDB[instID] = instance.Config{
			Services: map[flux.ServiceID]instance.ServiceConfig{
				serviceID: instance.ServiceConfig{
					Automated: true,
				},
			},
		}
		followUps, err := a.handleAutomatedInstanceJob(nullLogger, job)
		if err != nil {
			t.Fatal(err)
		}
		// - It should check for newer versions of each image
		t.Error("TODO")

		// - It should reschedule itself
		// - There should be no releases
		if len(followUps) != 1 || followUps[0].Key != job.Key {
			t.Error("Unexpected follow-up jobs when automated services (with no new images available): %q", followUps)
		}
	}

	// When there are automated services (with new images available)
	{
		// - It should check for newer versions of each image (once)
		// - It should reschedule itself
		// - There should be a release for each new image
	}

	t.Error("TODO")
}

func TestHandleAutomatedInstanceJob_IgnoresUndefinedAutomatedServices(t *testing.T) {
	t.Error("TODO")
}

func TestHandleAutomatedInstanceJob_DeploysNonRunningAutomatedServices(t *testing.T) {
	t.Error("TODO")
}
