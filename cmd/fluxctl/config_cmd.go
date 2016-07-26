package main

import (
	"github.com/spf13/cobra"
)

type configOpts struct {
	*rootOpts
}

func newConfig(parent *rootOpts) *configOpts {
	return &configOpts{rootOpts: parent}
}

func (opts *configOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manipulate configuration files",
	}

	cmd.AddCommand(newConfigUpdate(opts).Command())

	return cmd
}
