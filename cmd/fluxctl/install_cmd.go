package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/install"
)

type installOpts struct {
	install.TemplateParameters
	outputDir string
}

func newInstall() *installOpts {
	return &installOpts{}
}

func (opts *installOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Print and tweak Kubernetes manifests needed to install Flux in a Cluster",
		Example: `# Install Flux and make it use Git repository git@github.com:<your username>/flux-get-started
fluxctl install --git-url 'git@github.com:<your username>/flux-get-started' --git-email=<your_git_email> | kubectl -f -`,
		RunE: opts.RunE,
	}
	cmd.Flags().StringVar(&opts.GitURL, "git-url", "",
		"URL of the Git repository to be used by Flux, e.g. git@github.com:<your username>/flux-get-started")
	cmd.Flags().StringVar(&opts.GitBranch, "git-branch", "master",
		"Git branch to be used by Flux")
	cmd.Flags().StringSliceVar(&opts.GitPaths, "git-paths", []string{},
		"relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().StringSliceVar(&opts.GitPaths, "git-path", []string{},
		"relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().StringVar(&opts.GitLabel, "git-label", "flux",
		"Git label to keep track of Flux's sync progress; overrides both --git-sync-tag and --git-notes-ref")
	cmd.Flags().StringVar(&opts.GitUser, "git-user", "Flux",
		"username to use as git committer")
	cmd.Flags().StringVar(&opts.GitEmail, "git-email", "",
		"email to use as git committer")
	cmd.Flags().BoolVar(&opts.GitReadOnly, "git-readonly", false,
		"tell flux it has readonly access to the repo")
	cmd.Flags().BoolVar(&opts.ManifestGeneration, "manifest-generation", false,
		"whether to enable manifest generation")
	cmd.Flags().StringVar(&opts.Namespace, "namespace", "",
		"cluster namespace where to install flux")
	cmd.Flags().BoolVar(&opts.RegistryDisableScanning, "registry-disable-scanning", false,
		"do not scan container image registries to fill in the registry cache")
	cmd.Flags().StringVarP(&opts.outputDir, "output-dir", "o", "", "a directory in which to write individual manifests, rather than printing to stdout")
	cmd.Flags().BoolVar(&opts.AddSecurityContext, "add-security-context", true, "Ensure security context information is added to the pod specs. Defaults to 'true'")

	// Hide and deprecate "git-paths", which was wrongly introduced since its inconsistent with fluxd's git-path flag
	cmd.Flags().MarkHidden("git-paths")
	cmd.Flags().MarkDeprecated("git-paths", "please use --git-path (no ending s) instead")

	return cmd
}

func (opts *installOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}
	if opts.GitURL == "" {
		return fmt.Errorf("please supply a valid --git-url argument")
	}
	if opts.GitEmail == "" {
		return fmt.Errorf("please supply a valid --git-email argument")
	}
	opts.TemplateParameters.Namespace = getKubeConfigContextNamespaceOrDefault(opts.Namespace, "default", "")
	manifests, err := install.FillInTemplates(opts.TemplateParameters)
	if err != nil {
		return err
	}

	writeManifest := func(fileName string, content []byte) error {
		_, err := os.Stdout.Write(content)
		return err
	}

	if opts.outputDir != "" {
		info, err := os.Stat(opts.outputDir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", opts.outputDir)
		}
		writeManifest = func(fileName string, content []byte) error {
			path := filepath.Join(opts.outputDir, fileName)
			fmt.Fprintf(os.Stderr, "writing %s\n", path)
			return ioutil.WriteFile(path, content, os.FileMode(0666))
		}
	}

	for fileName, content := range manifests {
		if err := writeManifest(fileName, content); err != nil {
			return fmt.Errorf("cannot output manifest file %s: %s", fileName, err)
		}
	}

	return nil
}
