package testfiles

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestWriteTestFiles(t *testing.T) {
	dir, cleanup := TempDir(t)
	defer cleanup()

	if err := WriteTestFiles(dir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	for file, contents := range Files {
		var bytes []byte
		var err error
		if bytes, err = ioutil.ReadFile(filepath.Join(dir, file)); err != nil {
			t.Error(err)
		}
		if string(bytes) != contents {
			t.Errorf("file %s has unexpected contents: %q", filepath.Join(dir, file), string(bytes))
		}
	}
}
