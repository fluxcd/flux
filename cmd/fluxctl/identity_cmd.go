package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type identityOpts struct {
	*rootOpts
	regenerate bool
}

func newIdentity(parent *rootOpts) *identityOpts {
	return &identityOpts{rootOpts: parent}
}

func (opts *identityOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Display SSH public key",
		RunE:  opts.RunE,
	}
	cmd.Flags().BoolVarP(&opts.regenerate, "regenerate", "r", false, `Generate a new identity`)
	return cmd
}

func (opts *identityOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	publicSSHKey, err := opts.API.PublicSSHKey(noInstanceID, opts.regenerate)
	if err != nil {
		return err
	}

	fmt.Print(publicSSHKey.Key)
	fmt.Println(publicSSHKey.Fingerprints["md5"].Hash)
	fmt.Print(publicSSHKey.Fingerprints["md5"].Randomart)

	return nil
}
