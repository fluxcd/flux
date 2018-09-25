package release

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	flux_v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
)

// fetchPath gives the filesystem location for a chart, without
// testing whether the file exists or not.
func fetchPath(base string, source *flux_v1beta1.RepoChartSource) string {
	// We don't need to obscure the location of the charts in the
	// filesystem; but we do need a stable, filesystem-friendly path
	// to them that is based on the URL.
	repoPath := filepath.Join(base, base64.URLEncoding.EncodeToString([]byte(source.RepoURL)))
	if err := os.MkdirAll(repoPath, os.FileMode(os.ModeDir+0770)); err != nil {
		panic(err)
	}
	filename := fmt.Sprintf("%s-%s.tar", source.Name, source.Version)
	return filepath.Join(repoPath, filename)
}

// ensureChartFetched returns the path to a downloaded chart, fetching
// it first if necessary. It always returns the expected path to the
// chart, and either an error or nil.
func ensureChartFetched(base string, source *flux_v1beta1.RepoChartSource) (string, error) {
	chartPath := fetchPath(base, source)
	stat, err := os.Stat(chartPath)
	switch {
	case os.IsNotExist(err):
		return downloadChart(chartPath, source)
	case err != nil:
		return chartPath, err
	case stat.IsDir():
		return chartPath, errors.New("path to chart exists but is a directory")
	}
	return chartPath, nil
}

// downloadChart fetches a chart tarball, given the name, version and
// repo URL in `source`.
func downloadChart(dest string, source *flux_v1beta1.RepoChartSource) (string, error) {
	// FIXME(michael): STUB!
	println("Downloading", dest)
	return dest, nil
}
