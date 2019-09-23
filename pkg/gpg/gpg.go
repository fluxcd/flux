// Package gpg has procedures for dealing with GNU Privacy Guard
// (gpg), in service of signing commits.
package gpg

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// ImportKeys looks for all keys in a directory, and imports them into
// the current user's keyring. A path to a directory or a file may be
// provided. If the path is a directory, regular files in the
// directory will be imported, but not subdirectories (i.e., no
// recursion). It returns the basenames of the succesfully imported
// keys.
func ImportKeys(src string, trustImportedKeys bool) ([]string, error) {
	info, err := os.Stat(src)
	var files []string
	switch {
	case err != nil:
		return nil, err
	case info.IsDir():
		infos, err := ioutil.ReadDir(src)
		if err != nil {
			return nil, err
		}
		for _, f := range infos {
			filepath := filepath.Join(src, f.Name())
			if f, err = os.Stat(filepath); err != nil {
				continue
			}
			if f.Mode().IsRegular() {
				files = append(files, filepath)
			}
		}
	default:
		files = []string{src}
	}

	var imported []string
	var failed []string
	for _, path := range files {
		if err := gpgImport(path); err != nil {
			failed = append(failed, filepath.Base(path))
			continue
		}
		imported = append(imported, filepath.Base(path))
	}

	if failed != nil {
		return imported, fmt.Errorf("errored importing keys: %v", failed)
	}

	if trustImportedKeys {
		if err = gpgTrustImportedKeys(); err != nil {
			return imported, err
		}
	}

	return imported, nil
}

func gpgImport(path string) error {
	cmd := exec.Command("gpg", "--import", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error importing key: %s", string(out))
	}
	return nil
}

func gpgTrustImportedKeys() error {
	// List imported keys and their fingerprints, grep the fingerprints,
	// transform them into a format gpg understands, and pipe the output
	// into --import-ownertrust.
	arg := `gpg --list-keys --fingerprint | grep pub -A 1 | egrep -Ev "pub|--"|tr -d ' ' | awk 'BEGIN { FS = "\n" } ; { print $1":6:" }' | gpg --import-ownertrust`
	cmd := exec.Command("sh", "-c", arg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error trusting imported keys: %s", string(out))
	}
	return nil
}
