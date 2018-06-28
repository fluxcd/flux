package resource

import (
	"testing"

	"github.com/weaveworks/flux/resource"
)

func TestParseSimpleFormat(t *testing.T) {
	expectedImage := "bitnami/mariadb:10.1.30-r1"
	doc := `---
apiVersion: helm.integrations.flux.weave.works/v1alpha2
kind: FluxHelmRelease
metadata:
  name: mariadb
  namespace: maria
  labels:
    chart: mariadb
spec:
  chartGitPath: mariadb
  values:
    image: ` + expectedImage + `
    persistence:
      enabled: false
`

	resources, err := ParseMultidoc([]byte(doc), "test")
	if err != nil {
		t.Fatal(err)
	}
	res, ok := resources["maria:fluxhelmrelease/mariadb"]
	if !ok {
		t.Fatalf("expected resource not found; instead got %#v", resources)
	}
	fhr, ok := res.(resource.Workload)
	if !ok {
		t.Fatalf("expected resource to be a Workload, instead got %#v", res)
	}

	containers := fhr.Containers()
	if len(containers) != 1 {
		t.Errorf("expected 1 container; got %#v", containers)
	}
	image := containers[0].Image.String()
	if image != expectedImage {
		t.Errorf("expected container image %q, got %q", expectedImage, image)
	}
}

func TestParseLessSimpleFormat(t *testing.T) {
	expectedContainer := "db"
	expectedImage := "bitnami/mariadb:10.1.30-r1"
	doc := `---
apiVersion: helm.integrations.flux.weave.works/v1alpha2
kind: FluxHelmRelease
metadata:
  name: mariadb
  namespace: maria
  labels:
    chart: mariadb
spec:
  chartGitPath: mariadb
  values:
    ` + expectedContainer + `:
      image: ` + expectedImage + `
      persistence:
        enabled: false
`

	resources, err := ParseMultidoc([]byte(doc), "test")
	if err != nil {
		t.Fatal(err)
	}
	res, ok := resources["maria:fluxhelmrelease/mariadb"]
	if !ok {
		t.Fatalf("expected resource not found; instead got %#v", resources)
	}
	fhr, ok := res.(resource.Workload)
	if !ok {
		t.Fatalf("expected resource to be a Workload, instead got %#v", res)
	}

	containers := fhr.Containers()
	if len(containers) != 1 {
		t.Fatalf("expected 1 container; got %#v", containers)
	}
	image := containers[0].Image.String()
	if image != expectedImage {
		t.Errorf("expected container image %q, got %q", expectedImage, image)
	}
	if containers[0].Name != expectedContainer {
		t.Errorf("expected container name %q, got %q", expectedContainer, containers[0].Name)
	}

	newImage := containers[0].Image.WithNewTag("some-other-tag")
	if err := fhr.SetContainerImage(expectedContainer, newImage); err != nil {
		t.Error(err)
	}

	containers = fhr.Containers()
	if len(containers) != 1 {
		t.Fatalf("expected 1 container; got %#v", containers)
	}
	image = containers[0].Image.String()
	if image != newImage.String() {
		t.Errorf("expected container image %q, got %q", newImage.String(), image)
	}
	if containers[0].Name != expectedContainer {
		t.Errorf("expected container name %q, got %q", expectedContainer, containers[0].Name)
	}
}
