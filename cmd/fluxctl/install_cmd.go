package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/install"
)

type installOpts struct {
	install.TemplateParameters
	outputDir  string
	configFile string
}

func newInstall() *installOpts {
	return &installOpts{}
}

func (opts *installOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Print and tweak Kubernetes manifests needed to install Flux in a Cluster",
		Example: `# Install Flux and make it use Git repository git@github.com:<your username>/flux-get-started
fluxctl install --git-url 'git@github.com:<your username>/flux-get-started' --git-email=<your_git_email> | kubectl -f -

# Install Flux using a local file for the config, and writing the manifests out to files in ./flux
fluxctl install --config-file ./flux-config.yaml -o ./flux/
`,
		RunE: opts.RunE,
	}
	cmd.Flags().StringVar(&opts.GitURL, "git-url", "",
		"URL of the Git repository to be used by Flux, e.g. git@github.com:<your username>/flux-get-started")
	cmd.Flags().StringVar(&opts.GitBranch, "git-branch", "master",
		"Git branch to be used by Flux")
	cmd.Flags().StringSliceVar(&opts.GitPaths, "git-path", []string{},
		"Relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().StringVar(&opts.GitLabel, "git-label", "flux",
		"Git label to keep track of Flux's sync progress; overrides both --git-sync-tag and --git-notes-ref")
	cmd.Flags().StringVar(&opts.GitUser, "git-user", "Flux", "Username to use as git committer")
	cmd.Flags().StringVar(&opts.GitEmail, "git-email", "", "Email to use as git committer")
	cmd.Flags().BoolVar(&opts.GitReadOnly, "git-readonly", false, "Tell flux it has readonly access to the repo")
	cmd.Flags().BoolVar(&opts.ManifestGeneration, "manifest-generation", false, "Whether to enable manifest generation")
	cmd.Flags().StringVar(&opts.Namespace, "namespace", "", "Cluster namespace in which to install Flux")
	cmd.Flags().BoolVar(&opts.RegistryDisableScanning, "registry-disable-scanning", false,
		"do not scan container image registries to fill in the registry cache")
	cmd.Flags().BoolVar(&opts.AddSecurityContext, "add-security-context", true, "Ensure security context information is added to the pod specs. Defaults to 'true'")

	cmd.Flags().StringVar(&opts.configFile, "config-file", "", "Config file used to configure Flux")
	cmd.Flags().BoolVar(&opts.ConfigAsConfigMap, "config-as-configmap", false,
		"Create a ConfigMap to hold the Flux configuration given with --config-file. If false, a Secret will be used. Ignored if --config-file is not given.")
	cmd.Flags().StringVarP(&opts.outputDir, "output-dir", "o", "", "A directory in which to write individual manifests, rather than printing to stdout")

	// Hide and deprecate "git-paths", which was wrongly introduced since its inconsistent with fluxd's git-path flag
	cmd.Flags().StringSliceVar(&opts.GitPaths, "git-paths", []string{},
		"Relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().MarkHidden("git-paths")
	cmd.Flags().MarkDeprecated("git-paths", "please use --git-path (no ending s) instead")

	return cmd
}

func (opts *installOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if opts.configFile == "" {
		if opts.GitURL == "" {
			return fmt.Errorf("please supply a valid --git-url argument")
		}
		if opts.GitEmail == "" {
			return fmt.Errorf("please supply a valid --git-email argument")
		}
	} else {
		configFileReader, err := os.Open(opts.configFile)
		if err != nil {
			return fmt.Errorf("unable to open flux config file: %s", err.Error())
		}
		opts.ConfigFileContent, err = install.ConfigContent(configFileReader, opts.ConfigAsConfigMap)
		if err != nil {
			return fmt.Errorf("unable to construct config resource: %s", err.Error())
		}
	}

	opts.Namespace = getKubeConfigContextNamespaceOrDefault(opts.Namespace, "default", "")

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
		// The way the templating works, it will run through each
		// file; but some may elide the entire content if it's not
		// needed. So, don't bother writing anything out if the
		// template evaluated to just whitespace.
		if len(bytes.TrimSpace(content)) == 0 {
			continue
		}

		if err := writeManifest(fileName, content); err != nil {
			return fmt.Errorf("cannot output manifest file %s: %s", fileName, err)
		}
	}

	return nil
}
