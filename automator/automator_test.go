package automator

import (
	"os"
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
	serviceID := flux.ServiceID("extra/pr-assigner")
	instanceDB := mockInstanceDB{
		instID: instance.Config{},
	}
	repoSource, err := git.NewMockRepo()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(repoSource)

	// TODO: We shouldn't need the service definition, just the deployment.
	for path, data := range map[string]string{
		"pr-assigner-svc.yml": prAssignerSvcYaml,
		"pr-assigner-dep.yml": prAssignerDepYaml,
	} {
		if err := git.AddFileToMockRepo(repoSource, path, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}

	instancer := instance.StandaloneInstancer{
		Instance:  instID,
		Connecter: platform.NewStandaloneMessageBus(platform.NewBusMetrics()),
		Registry:  nil, // *registry.Client
		Config:    instanceDB,
		GitRepo:   git.Repo{URL: "file://" + repoSource},
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

const prAssignerSvcYaml = `---
apiVersion: v1
kind: Service
metadata:
  name: pr-assigner
  namespace: extra
spec:
  ports:
    - port: 80
  selector:
    name: pr-assigner
`

const prAssignerDepYaml = `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: pr-assigner
  namespace: extra
spec:
  replicas: 1
  template:
    metadata:
      labels:
        name: pr-assigner
    spec:
      imagePullSecrets:
      - name: quay-secret
      containers:
        - name: pr-assigner
          image: quay.io/weaveworks/pr-assigner:master-6f5e816
          imagePullPolicy: IfNotPresent
          args:
            - --conf_path=/config/pr-assigner.json
          env:
            - name: GITHUB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: pr-assigner
                  key: githubtoken
          volumeMounts:
            - name: config-volume
              mountPath: /config
      volumes:
        - name: config-volume
          configMap:
            name: pr-assigner
            items:
              - key: conffile
                path: pr-assigner.json
`

func TestHandleAutomatedInstanceJob_CloningRepoFails(t *testing.T) {
	// It should send a notification and back-off
	t.Error("TODO")
}

func TestHandleAutomatedInstanceJob_IgnoresUndefinedAutomatedServices(t *testing.T) {
	t.Error("TODO")
}

func TestHandleAutomatedInstanceJob_DeploysNonRunningAutomatedServices(t *testing.T) {
	t.Error("TODO")
}
