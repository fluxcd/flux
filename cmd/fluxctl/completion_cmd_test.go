package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompletionCommand_InputFailure(t *testing.T) {
	for k, v := range [][]string{
		{},
		{"foo"},
		{"bash", "zsh", "fish"},
	} {
		t.Run(fmt.Sprintf("%d", k), func(t *testing.T) {
			cmd := newCompletionCommand()
			cmd.SetArgs(v)
			err := cmd.Execute()
			assert.Error(t, err)
		})
	}
}

func TestCompletionCommand_Success(t *testing.T) {
	for k, v := range [][]string{
		{"bash"},
		{"zsh"},
		{"fish"},
	} {
		t.Run(fmt.Sprintf("%d", k), func(t *testing.T) {
			parentCmd := newRoot().Command()
			cmd := newCompletionCommand()
			parentCmd.AddCommand(cmd)
			cmd.SetArgs(v)
			err := cmd.Execute()
			assert.NoError(t, err)
		})
	}
}
