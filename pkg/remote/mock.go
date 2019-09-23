package remote

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	"github.com/fluxcd/flux/pkg/guid"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

type MockServer struct {
	PingError error

	VersionAnswer string
	VersionError  error

	ExportAnswer []byte
	ExportError  error

	ListServicesAnswer []v6.ControllerStatus
	ListServicesError  error

	ListImagesAnswer []v6.ImageStatus
	ListImagesError  error

	UpdateManifestsArgTest func(update.Spec) error
	UpdateManifestsAnswer  job.ID
	UpdateManifestsError   error

	NotifyChangeError error

	SyncStatusAnswer []string
	SyncStatusError  error

	JobStatusAnswer job.Status
	JobStatusError  error

	GitRepoConfigAnswer v6.GitConfig
	GitRepoConfigError  error
}

func (p *MockServer) Ping(ctx context.Context) error {
	return p.PingError
}

func (p *MockServer) Version(ctx context.Context) (string, error) {
	return p.VersionAnswer, p.VersionError
}

func (p *MockServer) Export(ctx context.Context) ([]byte, error) {
	return p.ExportAnswer, p.ExportError
}

func (p *MockServer) ListServices(ctx context.Context, ns string) ([]v6.ControllerStatus, error) {
	return p.ListServicesAnswer, p.ListServicesError
}

func (p *MockServer) ListServicesWithOptions(context.Context, v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	return p.ListServicesAnswer, p.ListServicesError
}

func (p *MockServer) ListImages(context.Context, update.ResourceSpec) ([]v6.ImageStatus, error) {
	return p.ListImagesAnswer, p.ListImagesError
}

func (p *MockServer) ListImagesWithOptions(context.Context, v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	return p.ListImagesAnswer, p.ListImagesError
}

func (p *MockServer) UpdateManifests(ctx context.Context, s update.Spec) (job.ID, error) {
	if p.UpdateManifestsArgTest != nil {
		if err := p.UpdateManifestsArgTest(s); err != nil {
			return job.ID(""), err
		}
	}
	return p.UpdateManifestsAnswer, p.UpdateManifestsError
}

func (p *MockServer) NotifyChange(ctx context.Context, change v9.Change) error {
	return p.NotifyChangeError
}

func (p *MockServer) SyncStatus(context.Context, string) ([]string, error) {
	return p.SyncStatusAnswer, p.SyncStatusError
}

func (p *MockServer) JobStatus(context.Context, job.ID) (job.Status, error) {
	return p.JobStatusAnswer, p.JobStatusError
}

func (p *MockServer) GitRepoConfig(ctx context.Context, regenerate bool) (v6.GitConfig, error) {
	return p.GitRepoConfigAnswer, p.GitRepoConfigError
}

var _ api.Server = &MockServer{}

// -- Battery of tests for an api.Server implementation. Since these
// essentially wrap the server in various transports, we expect
// arguments and answers to be preserved.

func ServerTestBattery(t *testing.T, wrap func(mock api.Server) api.Server) {
	// set up
	namespace := "the-space-of-names"
	serviceID := resource.MustParseID(namespace + "/service")
	serviceList := []resource.ID{serviceID}
	services := resource.IDSet{}
	services.Add(serviceList)

	now := time.Now().UTC()

	imageID, _ := image.ParseRef("quay.io/example.com/frob:v0.4.5")
	serviceAnswer := []v6.ControllerStatus{
		v6.ControllerStatus{
			ID:     resource.MustParseID("foobar/hello"),
			Status: "ok",
			Containers: []v6.Container{
				v6.Container{
					Name: "frobnicator",
					Current: image.Info{
						ID:        imageID,
						CreatedAt: now,
					},
				},
			},
		},
		v6.ControllerStatus{},
	}

	imagesAnswer := []v6.ImageStatus{
		v6.ImageStatus{
			ID: resource.MustParseID("barfoo/yello"),
			Containers: []v6.Container{
				{
					Name: "flubnicator",
					Current: image.Info{
						ID: imageID,
					},
				},
			},
		},
	}

	syncStatusAnswer := []string{
		"commit 1",
		"commit 2",
		"commit 3",
	}

	updateSpec := update.Spec{
		Type: update.Images,
		Spec: update.ReleaseImageSpec{
			ServiceSpecs: []update.ResourceSpec{
				update.ResourceSpecAll,
			},
			ImageSpec: update.ImageSpecLatest,
		},
	}
	checkUpdateSpec := func(s update.Spec) error {
		if !reflect.DeepEqual(updateSpec, s) {
			return errors.New("expected != actual")
		}
		return nil
	}

	mock := &MockServer{
		ListServicesAnswer:     serviceAnswer,
		ListImagesAnswer:       imagesAnswer,
		UpdateManifestsArgTest: checkUpdateSpec,
		UpdateManifestsAnswer:  job.ID(guid.New()),
		SyncStatusAnswer:       syncStatusAnswer,
	}

	ctx := context.Background()

	// OK, here we go
	client := wrap(mock)

	if err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	ss, err := client.ListServices(ctx, namespace)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ss, mock.ListServicesAnswer) {
		t.Error(fmt.Errorf("expected:\n%#v\ngot:\n%#v", mock.ListServicesAnswer, ss))
	}
	mock.ListServicesError = fmt.Errorf("list services query failure")
	ss, err = client.ListServices(ctx, namespace)
	if err == nil {
		t.Error("expected error from ListServices, got nil")
	}

	ims, err := client.ListImagesWithOptions(ctx, v10.ListImagesOptions{
		Spec: update.ResourceSpecAll,
	})
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ims, mock.ListImagesAnswer) {
		t.Error(fmt.Errorf("expected:\n%#v\ngot:\n%#v", mock.ListImagesAnswer, ims))
	}
	mock.ListImagesError = fmt.Errorf("list images error")
	if _, err = client.ListImagesWithOptions(ctx, v10.ListImagesOptions{
		Spec: update.ResourceSpecAll,
	}); err == nil {
		t.Error("expected error from ListImages, got nil")
	}

	jobid, err := mock.UpdateManifests(ctx, updateSpec)
	if err != nil {
		t.Error(err)
	}
	if jobid != mock.UpdateManifestsAnswer {
		t.Error(fmt.Errorf("expected %q, got %q", mock.UpdateManifestsAnswer, jobid))
	}
	mock.UpdateManifestsError = fmt.Errorf("update manifests error")
	if _, err = client.UpdateManifests(ctx, updateSpec); err == nil {
		t.Error("expected error from UpdateManifests, got nil")
	}

	change := v9.Change{Kind: v9.GitChange, Source: v9.GitUpdate{URL: "git@example.com:foo/bar"}}
	if err := client.NotifyChange(ctx, change); err != nil {
		t.Error(err)
	}

	syncSt, err := client.SyncStatus(ctx, "HEAD")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(mock.SyncStatusAnswer, syncSt) {
		t.Errorf("expected: %#v\ngot: %#v", mock.SyncStatusAnswer, syncSt)
	}
}
