package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type identityOpts struct {
	*rootOpts
	regenerate  bool
	fingerprint bool
	visual      bool
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
	cmd.Flags().BoolVarP(&opts.fingerprint, "fingerprint", "l", false, `Show fingerprint of public key`)
	cmd.Flags().BoolVarP(&opts.visual, "visual", "v", false, `Show ASCII art representation with fingerprint (implies -l)`)
	return cmd
}

func (opts *identityOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	ctx := context.Background()

	repoConfig, err := opts.API.GitRepoConfig(ctx, opts.regenerate)
	if err != nil {
		return err
	}
	publicSSHKey := repoConfig.PublicSSHKey

	if opts.visual {
		opts.fingerprint = true
	}

	if opts.fingerprint {
		fmt.Println(publicSSHKey.Fingerprints["md5"].Hash)
		if opts.visual {
			fmt.Print(publicSSHKey.Fingerprints["md5"].Randomart)
		}
	} else {
		fmt.Print(publicSSHKey.Key)
	}
	return nil
}
