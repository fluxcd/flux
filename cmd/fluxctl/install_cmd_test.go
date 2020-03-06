package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestInstallCommand_ExtraArgumentFailure(t *testing.T) {
	for k, v := range [][]string{
		{"foo"},
		{"foo", "bar"},
		{"foo", "bar", "bizz"},
		{"foo", "bar", "bizz", "buzz"},
	} {
		t.Run(fmt.Sprintf("%d", k), func(t *testing.T) {
			cmd := newInstall().Command()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs(v)
			_ = cmd.Flags().Set("git-url", "git@github.com:testcase/flux-get-started")
			_ = cmd.Flags().Set("git-email", "testcase@weave.works")
			if err := cmd.Execute(); err == nil {
				t.Fatalf("expecting error, got nil")
			}
		})
	}
}

func TestInstallCommand_MissingRequiredFlag(t *testing.T) {
	for k, v := range map[string]string{
		"git-url":   "git@github.com:testcase/flux-get-started",
		"git-email": "testcase@weave.works",
	} {
		t.Run(fmt.Sprintf("only --%s", k), func(t *testing.T) {
			cmd := newInstall().Command()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{})
			_ = cmd.Flags().Set(k, v)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("expecting error, got nil")
			}
		})
	}
}

func TestInstallCommand_Success(t *testing.T) {
	f := make(map[string]string)
	f["git-url"] = "git@github.com:testcase/flux-get-started"
	f["git-email"] = "testcase@weave.works"

	cmd := newInstall().Command()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})
	for k, v := range f {
		_ = cmd.Flags().Set(k, v)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expecting nil, got error (%s)", err.Error())
	}
}
