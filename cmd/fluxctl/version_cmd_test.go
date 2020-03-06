package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestVersionCommand_InputFailure(t *testing.T) {
	for k, v := range [][]string{
		{"foo"},
		{"foo", "bar"},
		{"foo", "bar", "bizz"},
		{"foo", "bar", "bizz", "buzz"},
	} {
		t.Run(fmt.Sprintf("%d", k), func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := newVersionCommand()
			cmd.SetOut(buf)
			cmd.SetArgs(v)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("Expecting error: command is not expecting extra arguments")
			}
		})
	}
}

func TestVersionCommand_Success(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newVersionCommand()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Expecting nil, got error (%s)", err.Error())
	}
}

func TestVersionCommand_SuccessCheckVersion(t *testing.T) {
	for _, e := range []string{
		"v1.0.0",
		"v2.0.0",
	} {
		t.Run(e, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := newVersionCommand()
			version = e
			cmd.SetOut(buf)
			cmd.SetArgs([]string{})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Expecting nil, got error (%s)", err.Error())
			}
			if g := strings.TrimRight(buf.String(), "\n"); e != g {
				println(e == g)
				t.Fatalf("Expected %s, got %s", e, g)
			}
		})
	}
}
