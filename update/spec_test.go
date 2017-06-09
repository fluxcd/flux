package update

import "testing"

func TestParseImageSpec(t *testing.T) {
	parseSpec(t, "valid/image:tag", false)
	parseSpec(t, "image:tag", false)
	parseSpec(t, ":tag", true)
	parseSpec(t, "image:", true)
	parseSpec(t, "image", true)
	parseSpec(t, string(ImageSpecLatest), false)
	parseSpec(t, "<invalid spec>", true)
}

func parseSpec(t *testing.T, image string, expectError bool) {
	spec, err := ParseImageSpec(image)
	isErr := (err != nil)
	if isErr != expectError {
		t.Fatalf("Expected error = %v for %q. Error = %q\n", expectError, image, err)
	}
	if !expectError && (string(spec) != image) {
		t.Fatalf("Expected string spec %q but got %q", image, string(spec))
	}
}
