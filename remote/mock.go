package remote

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type MockPlatform struct {
	PingError error

	VersionAnswer string
	VersionError  error

	ExportAnswer []byte
	ExportError  error

	ListServicesAnswer []flux.ServiceStatus
	ListServicesError  error

	ListImagesAnswer []flux.ImageStatus
	ListImagesError  error

	UpdateManifestsArgTest func(update.Spec) error
	UpdateManifestsAnswer  job.ID
	UpdateManifestsError   error

	SyncNotifyError error

	SyncStatusAnswer []string
	SyncStatusError  error
}

func (p *MockPlatform) Ping() error {
	return p.PingError
}

func (p *MockPlatform) Version() (string, error) {
	return p.VersionAnswer, p.VersionError
}

func (p *MockPlatform) Export() ([]byte, error) {
	return p.ExportAnswer, p.ExportError
}

func (p *MockPlatform) ListServices(ns string) ([]flux.ServiceStatus, error) {
	return p.ListServicesAnswer, p.ListServicesError
}

func (p *MockPlatform) ListImages(flux.ServiceSpec) ([]flux.ImageStatus, error) {
	return p.ListImagesAnswer, p.ListImagesError
}

func (p *MockPlatform) UpdateManifests(s update.Spec) (job.ID, error) {
	if p.UpdateManifestsArgTest != nil {
		if err := p.UpdateManifestsArgTest(s); err != nil {
			return job.ID(""), err
		}
	}
	return p.UpdateManifestsAnswer, p.UpdateManifestsError
}

func (p *MockPlatform) SyncNotify() error {
	return p.SyncNotifyError
}

func (p *MockPlatform) SyncStatus(string) ([]string, error) {
	return p.SyncStatusAnswer, p.SyncStatusError
}

var _ Platform = &MockPlatform{}

// -- Battery of tests for a platform mechanism. Since these
// essentially wrap the platform in various transports, we expect
// arguments and answers to be preserved.

func PlatformTestBattery(t *testing.T, wrap func(mock Platform) Platform) {
	// set up
	namespace := "the-space-of-names"
	serviceID := flux.ServiceID(namespace + "/service")
	serviceList := []flux.ServiceID{serviceID}
	services := flux.ServiceIDSet{}
	services.Add(serviceList)

	now := time.Now()

	imageID, _ := flux.ParseImageID("quay.io/example.com/frob:v0.4.5")
	serviceAnswer := []flux.ServiceStatus{
		flux.ServiceStatus{
			ID:     flux.ServiceID("foobar/hello"),
			Status: "ok",
			Containers: []flux.Container{
				flux.Container{
					Name: "frobnicator",
					Current: flux.ImageDescription{
						ID:        imageID,
						CreatedAt: &now,
					},
				},
			},
		},
		flux.ServiceStatus{},
	}

	imagesAnswer := []flux.ImageStatus{
		flux.ImageStatus{
			ID:         flux.ServiceID("barfoo/yello"),
			Containers: []flux.Container{},
		},
	}

	syncStatusAnswer := []string{
		"commit 1",
		"commit 2",
		"commit 3",
	}

	updateSpec := update.Spec{
		Type: update.Images,
		Spec: flux.ReleaseSpec{
			ServiceSpecs: []flux.ServiceSpec{
				flux.ServiceSpecAll,
			},
			ImageSpec: flux.ImageSpecLatest,
		},
	}
	checkUpdateSpec := func(s update.Spec) error {
		if !reflect.DeepEqual(updateSpec, s) {
			return errors.New("expected != actual")
		}
		return nil
	}

	mock := &MockPlatform{
		ListServicesAnswer:     serviceAnswer,
		ListImagesAnswer:       imagesAnswer,
		UpdateManifestsArgTest: checkUpdateSpec,
		UpdateManifestsAnswer:  job.ID(guid.New()),
		SyncStatusAnswer:       syncStatusAnswer,
	}

	// OK, here we go
	client := wrap(mock)

	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}

	ss, err := client.ListServices(namespace)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ss, mock.ListServicesAnswer) {
		t.Error(fmt.Errorf("expected %d result(s), got %+v", len(mock.ListServicesAnswer), ss))
	}
	mock.ListServicesError = fmt.Errorf("list services query failure")
	ss, err = client.ListServices(namespace)
	if err == nil {
		t.Error("expected error from ListServices, got nil")
	}

	ims, err := client.ListImages(flux.ServiceSpecAll)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ims, mock.ListImagesAnswer) {
		t.Error(fmt.Errorf("expected:\n%#v\ngot:\n%#v", mock.ListImagesAnswer, ims))
	}
	mock.ListImagesError = fmt.Errorf("list images error")
	if _, err = client.ListImages(flux.ServiceSpecAll); err == nil {
		t.Error("expected error from ListImages, got nil")
	}

	jobid, err := mock.UpdateManifests(updateSpec)
	if err != nil {
		t.Error(err)
	}
	if jobid != mock.UpdateManifestsAnswer {
		t.Error(fmt.Errorf("expected %q, got %q", mock.UpdateManifestsAnswer, jobid))
	}

	if err := client.SyncNotify(); err != nil {
		t.Error(err)
	}

	syncSt, err := client.SyncStatus("HEAD")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(mock.SyncStatusAnswer, syncSt) {
		t.Error(fmt.Errorf("expected: %#v\ngot: %#v"), mock.SyncStatusAnswer, syncSt)
	}
}
