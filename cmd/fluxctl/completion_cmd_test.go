package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompletionCommand_InputFailure(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected error
	}{
		{
			name : "no argument",
			args : []string{},
			expected: fmt.Errorf("please specify a shell"),
		},
		{
			name : "invalid shell option",
			args : []string{"foo"},
			expected: fmt.Errorf("unsupported shell type \"foo\""),
		},
		{
			name : "multiple shell options",
			args : []string{"bash", "zsh", "fish"},
			expected: fmt.Errorf("please specify one of the following shells: bash fish zsh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCompletionCommand()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			assert.Error(t, err)
			assert.Equal(t, tt.expected.Error(), err.Error())
		})
	}
}

func TestCompletionCommand_Success(t *testing.T) {
	tests := []struct {
		shell    string
		expected string
	}{
		{
			shell:    "bash",
			expected: "bash completion for completion",
		},
		{
			shell:    "zsh",
			expected: "compdef _completion completion",
		},
		{
			shell:    "fish",
			expected: "fish completion for completion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			cmd := newCompletionCommand()
			buf := new(bytes.Buffer)
			cmd.SetArgs([]string{tt.shell})
			cmd.SetOut(buf)
			err := cmd.Execute()
			assert.NoError(t, err)
			assert.Contains(t, buf.String(), tt.expected)
		})
	}
}
