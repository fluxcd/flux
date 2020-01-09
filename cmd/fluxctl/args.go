package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/update"
)

func AddCauseFlags(cmd *cobra.Command, opts *update.Cause) {
	username := getCommitAuthor()

	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", "attach a message to the update")
	cmd.Flags().StringVar(&opts.User, "user", username, "override the user reported as initiating the update")
}

func getCommitAuthor() string {
	userName := getUserGitConfigValue("user.name")
	userEmail := getUserGitConfigValue("user.email")

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

var execCommand = exec.Command

func getUserGitConfigValue(arg string) string {
	var out bytes.Buffer
	cmd := execCommand("git", "config", "--get", "--null", arg)
	cmd.Stdout = &out
	cmd.Stderr = ioutil.Discard

	err := cmd.Run()
	if err != nil {
		return ""
	}
	res := out.String()
	return strings.Trim(res, "\x00")
}

// Wrapper for getKubeConfigContextNamespace where the return order is
// 1. Given namespace
// 2. Namespace from current kubeconfig context if 1. is empty
// 3. Default namespace specified if 1 is empty and 2 fails
func getKubeConfigContextNamespaceOrDefault(namespace string, defaultNamespace string, kubeConfigContext string) string {
	if namespace == "" {
		return getKubeConfigContextNamespace(defaultNamespace, kubeConfigContext)
	}
	return namespace
}

var getKubeConfigContextNamespace = func(defaultNamespace string, kubeConfigContext string) string {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return defaultNamespace
	}

	var cc string
	if kubeConfigContext == "" {
		cc = config.CurrentContext
	} else {
		cc = kubeConfigContext
	}

	if c, ok := config.Contexts[cc]; ok && c.Namespace != "" {
		return c.Namespace
	}

	return defaultNamespace
}
