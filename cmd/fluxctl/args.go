package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/update"
)

func parseServiceOption(s string) (update.ServiceSpec, error) {
	if s == "" {
		return update.ServiceSpecAll, nil
	}
	return update.ParseServiceSpec(s)
}

func AddCauseFlags(cmd *cobra.Command, opts *update.Cause) {
	authorInfo := getUserGitconfig()
	username := getCommitAuthor(authorInfo)

	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", "attach a message to the update")
	cmd.Flags().StringVar(&opts.User, "user", username, "override the user reported as initiating the update")
}

func getCommitAuthor(authorInfo map[string]string) string {
	userName := authorInfo["user.name"]
	userEmail := authorInfo["user.email"]

	switch {
	case userName != "" && userEmail != "":
		return fmt.Sprintf("%s <%s>", userName, userEmail)
	case userEmail != "":
		return userEmail
	case userName != "":
		return userName
	}
	return ""
}

func getUserGitconfig() map[string]string {
	var out bytes.Buffer
	userGitconfig := make(map[string]string)
	cmd := exec.Command("git", "config", "--list")
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return userGitconfig
	}
	res := out.String()
	return userGitconfigMap(res)
}

func userGitconfigMap(s string) map[string]string {
	c := make(map[string]string)
	lines := splitList(s)
	for _, l := range lines {
		if l == "" {
			continue
		}
		prop := strings.SplitN(l, "=", 2)
		p := strings.TrimSpace(prop[0])
		v := strings.TrimSpace(prop[1])
		c[p] = v
	}
	return c
}

func splitList(s string) []string {
	outStr := strings.TrimSpace(s)
	if outStr == "" {
		return []string{}
	}
	return strings.Split(outStr, "\n")
}
