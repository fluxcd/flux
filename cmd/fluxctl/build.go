package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/spf13/cobra"
	disco "k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"

	kube "github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/manifests"
)

type buildOpts struct {
	base       string
	paths      []string
	generation bool   // use manifest generation
	defaultNS  string // override the default namespace
}

func newBuild() *buildOpts {
	return &buildOpts{}
}

func (opts *buildOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Generate manifests from a local directory as fluxd would",
		RunE:  opts.RunE,
	}

	cmd.Flags().StringVarP(&opts.base, "base", "C", ".", "run as if invoked in the directory given; the working directory is treated as the root, for the purpose of interpreting config")
	cmd.Flags().StringSliceVarP(&opts.paths, "path", "p", nil, "use the path(s) given as target paths to be examined; if not supplied, the root directory is assumed as the sole target path")
	cmd.Flags().BoolVarP(&opts.generation, "manifest-generation", "g", false, "act the same as fluxd with --manifest-generation; i.e., look for .flux.yaml files in the target path(s) and use the instructions there")
	cmd.Flags().StringVar(&opts.defaultNS, "default-namespace", "", "give the default namespace explicitly, rather than taking it from KUBE_CONFIG")

	return cmd
}

func (opts *buildOpts) RunE(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "building in %s\n", opts.base)

	root, err := filepath.Abs(opts.base)
	if err != nil {
		return err
	}

	_, err = os.Stat(root)
	if err != nil {
		return err
	}

	_, err = os.Stat(filepath.Join(root, ".git"))
	switch {
	case err == nil:
		break
	case os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "warning: no .git found in %s; typical use is to run in the git repo you use for fluxd (so, make sure you're deliberately running outside a git repo).\n", root)
	default:
		return err
	}

	if len(opts.paths) == 0 {
		opts.paths = []string{"."}
	}

	// We want absolute paths to give to manifest generation
	var absolutePaths []string
	for _, p := range opts.paths {
		absolutePaths = append(absolutePaths, filepath.Join(root, p))
	}

	// We need a connection to the cluster, to be able to answer the
	// question of whether a particular kind of resource is supposed
	// to have a namespace or not. TODO(michael): alternatively, load
	// all the static descriptions we can, and maybe allow people to
	// point at CRDs or specify their scope explicitly.
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "connecting to Kubernetes cluster %s...", config.Host)
	client, err := disco.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "connected\n")

	namespacer, err := kube.NewNamespacer(client, opts.defaultNS)
	if err != nil {
		return err
	}

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	kubeManifests := kube.NewManifests(namespacer, logger)

	var store manifests.Store
	if opts.generation {
		store, err = manifests.NewConfigAware(root, absolutePaths, kubeManifests)
		if err != nil {
			return err
		}
	} else {
		store = manifests.NewRawFiles(root, absolutePaths, kubeManifests)
	}

	fmt.Fprintf(os.Stderr, "generating manifests...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	all, err := store.GetAllResourcesByID(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "done")

	for id := range all {
		fmt.Println(id)
	}

	return nil
}
