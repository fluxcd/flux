package main

import (
	"fmt"
	"testing"
)

func TestCompletionCommand_InputFailure(t *testing.T) {
	for k, v := range [][]string{
		{},
		{"foo"},
		{"bash", "zsh"},
	} {
		t.Run(fmt.Sprintf("%d", k), func(t *testing.T) {
			cmd := newCompletionCommand()
			cmd.SetArgs(v)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("Expecting error: command is expecting either bash or zsh")
			}
		})
	}
}

func TestCompletionCommand_Success(t *testing.T) {
	for k, v := range [][]string{
		{"bash"},
		{"zsh"},
	} {
		t.Run(fmt.Sprintf("%d", k), func(t *testing.T) {
			parentCmd := newRoot().Command()
			cmd := newCompletionCommand()
			parentCmd.AddCommand(cmd)
			cmd.SetArgs(v)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Expecting nil, got error (%s)", err.Error())
			}
		})
	}
}
