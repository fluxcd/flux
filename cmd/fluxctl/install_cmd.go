package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/install"
)

type installOpts install.TemplateParameters

func newInstall() *installOpts {
	return &installOpts{}
}

func (opts *installOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Print and tweak Kubernetes manifests needed to install Flux in a Cluster",
		Example: `# Install Flux and make it use Git repository git@github.com:<your username>/flux-get-started
fluxctl install --git-url 'git@github.com:<your username>/flux-get-started' | kubectl -f -`,
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.GitURL, "git-url", "", "",
		"URL of the Git repository to be used by Flux, e.g. git@github.com:<your username>/flux-get-started")
	cmd.Flags().StringVarP(&opts.GitBranch, "git-branch", "", "master",
		"Git branch to be used by Flux")
	cmd.Flags().StringSliceVarP(&opts.GitPaths, "git-paths", "", []string{},
		"Relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().StringSliceVarP(&opts.GitPaths, "git-path", "", []string{},
		"Relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().StringVarP(&opts.GitLabel, "git-label", "", "flux",
		"Git label to keep track of Flux's sync progress; overrides both --git-sync-tag and --git-notes-ref")
	cmd.Flags().StringVarP(&opts.GitUser, "git-user", "", "Flux",
		"Username to use as git committer")
	cmd.Flags().StringVarP(&opts.ConfigFile, "config-file", "", "",
		"Config file used to configure Flux")
	cmd.Flags().BoolVarP(&opts.ConfigAsConfigMap, "config-as-configmap", "", false,
		"Create a ConfigMap to hold the Flux configuration. If false, a secret is used to hold the FLux configuration. ")
	cmd.Flags().StringVarP(&opts.GitEmail, "git-email", "", "",
		"Email to use as git committer")
	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "", getKubeConfigContextNamespace("default"),
		"Cluster namespace where to install flux")

	// Hide and deprecate "git-paths", which was wrongly introduced since its inconsistent with fluxd's git-path flag
	cmd.Flags().MarkHidden("git-paths")
	cmd.Flags().MarkDeprecated("git-paths", "please use --git-path (no ending s) instead")

	return cmd
}

func (opts *installOpts) RunE(cmd *cobra.Command, args []string) error {
	if opts.ConfigFile == "" {
		if opts.GitURL == "" {
			return fmt.Errorf("please supply a valid --git-url argument")
		}
		if opts.GitEmail == "" {
			return fmt.Errorf("please supply a valid --git-email argument")
		}
	} else {
		configFileReader, err := os.Open(opts.ConfigFile)
		if err != nil {
			return fmt.Errorf("unable to open flux config file: %s", err.Error())
		}
		opts.ConfigFileReader = configFileReader
	}

	manifests, err := install.FillInTemplates(install.TemplateParameters(*opts))
	if err != nil {
		return err
	}
	for fileName, content := range manifests {
		if _, err := os.Stdout.Write(content); err != nil {
			return fmt.Errorf("cannot output manifest file %s: %s", fileName, err)
		}

	}

	return nil
}
