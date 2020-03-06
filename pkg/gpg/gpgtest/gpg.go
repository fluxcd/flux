package gpgtest

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
)

// ImportGPGKey imports a gpg key into a temporary home directory. It returns
// the gpg home directory and a cleanup function to be called after the caller
// is finished with this key.
func ImportGPGKey(t *testing.T, key string) (string, func()){
	newDir, cleanup := testfiles.TempDir(t)

	cmd := exec.Command("gpg", "--homedir", newDir, "--import", "--")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	io.WriteString(stdin, key)
	stdin.Close()

	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return newDir, cleanup
}

// GPGKey creates a new, temporary GPG home directory and a public/private key
// pair. It returns the GPG home directory, the ID of the created key, and a
// cleanup function to be called after the caller is finished with this key.
// Since GPG uses /dev/random, this may block while waiting for entropy to
// become available.
func GPGKey(t *testing.T) (string, string, func()) {
	newDir, cleanup := testfiles.TempDir(t)

	cmd := exec.Command("gpg", "--homedir", newDir, "--batch", "--gen-key")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cleanup()
		t.Fatal(err)
	}

	io.WriteString(stdin, "Key-Type: DSA\n")
	io.WriteString(stdin, "Key-Length: 1024\n")
	io.WriteString(stdin, "Key-Usage: sign\n")
	io.WriteString(stdin, "Name-Real: Flux\n")
	io.WriteString(stdin, "Name-Email: flux@weave.works\n")
	io.WriteString(stdin, "%no-protection\n")
	stdin.Close()

	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatal(err)
	}

	gpgCmd := exec.Command("gpg", "--homedir", newDir, "--list-keys", "--with-colons", "--with-fingerprint")
	grepCmd := exec.Command("grep", "^fpr")
	cutCmd := exec.Command("cut", "-d:", "-f10")

	grepIn, gpgOut := io.Pipe()
	cutIn, grepOut := io.Pipe()
	var cutOut bytes.Buffer

	gpgCmd.Stdout = gpgOut
	grepCmd.Stdin, grepCmd.Stdout = grepIn, grepOut
	cutCmd.Stdin, cutCmd.Stdout = cutIn, &cutOut

	gpgCmd.Start()
	grepCmd.Start()
	cutCmd.Start()

	if err := gpgCmd.Wait(); err != nil {
		cleanup()
		t.Fatal(err)
	}
	gpgOut.Close()

	if err := grepCmd.Wait(); err != nil {
		cleanup()
		t.Fatal(err)
	}
	grepOut.Close()

	if err := cutCmd.Wait(); err != nil {
		cleanup()
		t.Fatal(err)
	}

	fingerprint := strings.TrimSpace(cutOut.String())
	return newDir, fingerprint, cleanup
}
