package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func helperCommand(command string, s ...string) (cmd *exec.Cmd) {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, s...)
	cmd = exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		t.Fatalf("No command\n")
	}

	_, args = args[0], args[1:]
	for _, a := range args {
		if a == "user.name" {
			fmt.Fprintf(os.Stdout, "Jane Doe")
		} else if a == "user.email" {
			fmt.Fprintf(os.Stdout, "jd@j.d")
		}
	}
}

func checkAuthor(t *testing.T, input string, expected string) {
	execCommand = helperCommand
	defer func() { execCommand = exec.Command }()
	author := getUserGitConfigValue(input)
	if author != expected {
		t.Fatalf("author %q does not match expected value %q", author, expected)
	}
}

func TestGetCommitAuthor_OnlyName(t *testing.T) {
	checkAuthor(t, "user.name", "Jane Doe")
}

func TestGetCommitAuthor_OnlyEmail(t *testing.T) {
	checkAuthor(t, "user.email", "jd@j.d")
}
