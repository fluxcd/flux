package main

import (
	"os/user"

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
	username := ""
	user, err := user.Current()
	if err == nil {
		username = user.Username
	}
	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", "attach a message to the update")
	cmd.Flags().StringVar(&opts.User, "user", username, "override the user reported as initating the update")
}
