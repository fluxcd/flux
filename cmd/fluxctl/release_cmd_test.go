package main //+integration

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

func TestReleaseCommand_CLIConversion(t *testing.T) {
	for _, v := range []struct {
		args         []string
		expectedSpec update.ReleaseImageSpec
	}{
		{[]string{"--update-all-images", "--all"}, update.ReleaseImageSpec{
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindExecute,
		}},
		{[]string{"--update-all-images", "--all", "--dry-run"}, update.ReleaseImageSpec{
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindPlan,
		}},
		{[]string{"--update-image=alpine:latest", "--all"}, update.ReleaseImageSpec{
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
			ImageSpec:    "alpine:latest",
			Kind:         update.ReleaseKindExecute,
		}},
		{[]string{"--update-all-images", "--workload=deployment/flux"}, update.ReleaseImageSpec{
			ServiceSpecs: []update.ResourceSpec{"default:deployment/flux"},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindExecute,
		}},
		{[]string{"--update-all-images", "--all", "--exclude=deployment/test,deployment/yeah"}, update.ReleaseImageSpec{
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindExecute,
			Excludes: []resource.ID{
				resource.MustParseID("default:deployment/test"),
				resource.MustParseID("default:deployment/yeah"),
			},
		}},
	} {
		svc := testArgs(t, v.args, false, "")

		// Check that UpdateManifests was called with correct body
		method := "UpdateManifests"
		if svc.calledURL(method) == nil {
			t.Fatalf("Expecting fluxctl to request %q, but did not.", method)
		}
		r := svc.calledRequest(method)
		var actualSpec update.Spec
		if err := json.NewDecoder(r.Body).Decode(&actualSpec); err != nil {
			t.Fatal("Failed to decode spec")
		}
		if !reflect.DeepEqual(v.expectedSpec, actualSpec.Spec) {
			t.Fatalf("Expected %#v but got %#v", v.expectedSpec, actualSpec.Spec)
		}

		// Check that GetRelease was polled for status
		method = "JobStatus"
		if svc.calledURL(method) == nil {
			t.Fatalf("Expecting fluxctl to request %q, but did not.", method)
		}
	}
}

func TestReleaseCommand_InputFailures(t *testing.T) {
	for _, v := range []struct {
		args []string
		msg  string
	}{
		{[]string{}, "Should error when no args"},
		{[]string{"--all"}, "Should error when not specifying image spec"},
		{[]string{"--all", "--update-image=alpine"}, "Should error with invalid image spec"},
		{[]string{"--update-all-images"}, "Should error when not specifying workload spec"},
		{[]string{"--workload=invalid&workload", "--update-all-images"}, "Should error with invalid workload"},
		{[]string{"subcommand"}, "Should error when given subcommand"},
	} {
		testArgs(t, v.args, true, v.msg)
	}

}
