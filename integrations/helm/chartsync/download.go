package chartsync

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"k8s.io/helm/pkg/getter"
	helmenv "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/repo"

	flux_v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
)

// makeChartPath gives the expected filesystem location for a chart,
// without testing whether the file exists or not.
func makeChartPath(base string, source *flux_v1beta1.RepoChartSource) string {
	// We don't need to obscure the location of the charts in the
	// filesystem; but we do need a stable, filesystem-friendly path
	// to them that is based on the URL.
	repoPath := filepath.Join(base, base64.URLEncoding.EncodeToString([]byte(source.RepoURL)))
	if err := os.MkdirAll(repoPath, os.FileMode(os.ModeDir+0660)); err != nil {
		panic(err)
	}
	filename := fmt.Sprintf("%s-%s.tgz", source.Name, source.Version)
	return filepath.Join(repoPath, filename)
}

// ensureChartFetched returns the path to a downloaded chart, fetching
// it first if necessary. It always returns the expected path to the
// chart, and either an error or nil.
func ensureChartFetched(base string, source *flux_v1beta1.RepoChartSource) (string, error) {
	chartPath := makeChartPath(base, source)
	stat, err := os.Stat(chartPath)
	switch {
	case os.IsNotExist(err):
		return chartPath, downloadChart(chartPath, source)
	case err != nil:
		return chartPath, err
	case stat.IsDir():
		return chartPath, errors.New("path to chart exists but is a directory")
	}
	return chartPath, nil
}

// downloadChart attempts to fetch a chart tarball, given the name,
// version and repo URL in `source`, and the path to write the file
// to in `destFile`.
func downloadChart(destFile string, source *flux_v1beta1.RepoChartSource) error {
	// Helm's support libs are designed to be driven by the
	// command-line client, so there are some inevitable CLI-isms,
	// like getting values from flags and the environment. None of
	// these things are directly relevant here, _except_ perhaps the
	// HELM_HOME environment entry. Since there's that possible
	// exception, we go through the ff (following faff).
	var settings helmenv.EnvSettings
	// Add the flag definitions ..
	flags := pflag.NewFlagSet("helm-env", pflag.ContinueOnError)
	settings.AddFlags(flags)
	// .. but we're not expecting any _actual_ flags, so there's no
	// Parse. This next bit will use any settings from the
	// environment.
	settings.Init(flags)
	getters := getter.All(settings) // <-- aaaand this is the payoff

	// This resolves the repo URL, chart name and chart version to a
	// URL for the chart.

	// TODO(michael): the cert file, key file, and CA file are left
	// blank, but could be supplied from the secret. NB these are
	// different to those used for accessing Tiller (which are in the
	// EnvSettings I believe).
	chartUrl, err := repo.FindChartInRepoURL(source.RepoURL, source.Name, source.Version, "", "", "", getters)
	if err != nil {
		return err
	}

	// Here I'm reproducing the useful part (for us) of
	// `k8s.io/helm/pkg/downloader.Downloader.ResolveChartVersion(...)`,
	// stepping around `DownloadTo(...)` as it's too general. The
	// former interacts with Helm's local caching, which would mean
	// having to maintain the local cache. Since we already have the
	// information we need, we can just go ahead and get the file.
	u, err := url.Parse(chartUrl)
	if err != nil {
		return err
	}
	getterConstructor, err := getters.ByScheme(u.Scheme)
	if err != nil {
		return err
	}
	// TODO(michael): here is where we'd use any cert/key/ca from the secret
	g, err := getterConstructor(chartUrl, "", "", "")
	// TODO(michael): here is where we'd use username/passwd from the secret
	// if t, ok := g.(*getter.HttpGetter); ok {
	// 	t.SetCredentials(username, passwd)
	// }

	chartBytes, err := g.Get(u.String())
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(destFile, chartBytes.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
