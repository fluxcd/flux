package flux

import "testing"

func TestFilesFor(t *testing.T) {
	files, err := filesFor("testdata", ServiceID("default/helloworld"))
	if err != nil {
		t.Fatal(err)
	}
	if want, have := 2, len(files); want != have {
		t.Fatalf("want %d, have %d", want, have)
	}
	if want, have := "testdata/helloworld-deploy.yaml", files[0]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}
	if want, have := "testdata/helloworld-rc.yaml", files[1]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}
}
