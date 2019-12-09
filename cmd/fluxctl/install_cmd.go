package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/fluxcd/flux/pkg/install"
)

type installOpts struct {
	params     install.TemplateParameters
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

	// These control what goes into the configuration of fluxd
	cmd.Flags().StringVar(&opts.params.GitURL, "git-url", "",
		"URL of the Git repository to be used by Flux, e.g. git@github.com:<your username>/flux-get-started")
	cmd.Flags().StringVar(&opts.params.GitBranch, "git-branch", "master",
		"Git branch to be used by Flux")
	cmd.Flags().StringSliceVar(&opts.params.GitPaths, "git-path", []string{},
		"Relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().StringVar(&opts.params.GitLabel, "git-label", "flux",
		"Git label to keep track of Flux's sync progress; overrides both --git-sync-tag and --git-notes-ref")
	cmd.Flags().StringVar(&opts.params.GitUser, "git-user", "Flux", "Username to use as git committer")
	cmd.Flags().StringVar(&opts.params.GitEmail, "git-email", "", "Email to use as git committer")
	cmd.Flags().BoolVar(&opts.params.ConfigAsConfigMap, "config-as-configmap", false,
		"Create a ConfigMap to hold the Flux configuration given with --config-file. If false, a Secret will be used. Ignored if --config-file is not given.")
	cmd.Flags().BoolVar(&opts.params.GitReadOnly, "git-readonly", false, "Tell flux it has readonly access to the repo")
	cmd.Flags().BoolVar(&opts.params.ManifestGeneration, "manifest-generation", false, "Whether to enable manifest generation")
	cmd.Flags().StringVar(&opts.params.Namespace, "namespace", "", "Cluster namespace in which to install Flux")

	cmd.Flags().BoolVar(&opts.params.RegistryDisableScanning, "registry-disable-scanning", false, "do not scan container image registries to fill in the registry cache")
	cmd.Flags().BoolVar(&opts.params.AddSecurityContext, "add-security-context", true, "Ensure security context information is added to the pod specs. Defaults to 'true'")

	// These are flags for control of the output, etc.
	cmd.Flags().StringVar(&opts.configFile, "config-file", "", "Make this file into a secret or configmap for Flux to mount as config")
	cmd.Flags().StringVarP(&opts.outputDir, "output-dir", "o", "", "A directory in which to write individual manifests, rather than printing to stdout")

	// Hide and deprecate "git-paths", which was wrongly introduced since its inconsistent with fluxd's git-path flag
	cmd.Flags().StringSliceVar(&opts.params.GitPaths, "git-paths", []string{},
		"Relative paths within the Git repo for Flux to locate Kubernetes manifests")
	cmd.Flags().MarkHidden("git-paths")
	cmd.Flags().MarkDeprecated("git-paths", "please use --git-path (no ending s) instead")

	return cmd
}

func (opts *installOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	// To make sure the mandatory flags are set, we mimic what fluxd
	// will do, and load from both the config file (if supplied, it
	// will be mounted into the fluxd container as a configmap or
	// secret) and the flags (which will go into the template as fluxd
	// args).

	if opts.configFile != "" {
		viper.SetConfigFile(opts.configFile)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("unable to read config at %s (possibly a missing file extension, try .yaml or .json): %s", opts.configFile, err)
		}
	}
	viper.BindPFlags(cmd.Flags())

	// check that our mandatory flags were set, either on the command
	// line or in the config file
	mandatoryFlags := []string{"git-url", "git-email"}
	var missingFlags []string
	for _, flag := range mandatoryFlags {
		if !(viper.InConfig(flag) || cmd.Flags().Changed(flag)) {
			missingFlags = append(missingFlags, flag)
		}
	}
	if len(missingFlags) > 0 {
		return fmt.Errorf("(each of) %s must be set either by command-line flag, or in a file supplied to --config-file",
			strings.Join(missingFlags, ", "))
	}

	if opts.configFile != "" {
		configFileReader, err := os.Open(opts.configFile)
		if err != nil {
			return fmt.Errorf("unable to open flux config file: %s", err.Error())
		}
		opts.params.ConfigFileContent, err = install.ConfigContent(configFileReader, opts.params.ConfigAsConfigMap)
		if err != nil {
			return fmt.Errorf("unable to construct config resource: %s", err.Error())
		}
	}

	opts.params.Namespace = getKubeConfigContextNamespaceOrDefault(opts.params.Namespace, "default", "")
	manifests, err := install.FillInTemplates(opts.params)
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
