package resource

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Load takes paths to directories or files, and creates an object set
// based on the file(s) therein. Resources are named according to the
// file content, rather than the file name of directory structure.
func Load(base string, paths []string) (map[string]KubeManifest, error) {
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, fmt.Errorf("git path %q not found", base)
	}
	objs := map[string]KubeManifest{}
	charts, err := newChartTracker(base)
	if err != nil {
		return nil, errors.Wrapf(err, "walking %q for chartdirs", base)
	}
	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.Wrapf(err, "walking %q for yamels", path)
			}

			if charts.isDirChart(path) {
				return filepath.SkipDir
			}

			if charts.isPathInChart(path) {
				return nil
			}

			if !info.IsDir() && filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
				bytes, err := ioutil.ReadFile(path)
				if err != nil {
					return errors.Wrapf(err, "unable to read file at %q", path)
				}
				source, err := filepath.Rel(base, path)
				if err != nil {
					return errors.Wrapf(err, "path to scan %q is not under base %q", path, base)
				}
				docsInFile, err := ParseMultidoc(bytes, source)
				if err != nil {
					return err
				}
				for id, obj := range docsInFile {
					if alreadyDefined, ok := objs[id]; ok {
						return fmt.Errorf(`duplicate definition of '%s' (in %s and %s)`, id, alreadyDefined.Source(), source)
					}
					objs[id] = obj
				}
			}
			return nil
		})
		if err != nil {
			return objs, err
		}
	}

	return objs, nil
}

type chartTracker map[string]bool

func newChartTracker(root string) (chartTracker, error) {
	var chartdirs = make(map[string]bool)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "walking %q for charts", path)
		}

		if info.IsDir() && looksLikeChart(path) {
			chartdirs[path] = true
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return chartTracker(chartdirs), nil
}

func (c chartTracker) isDirChart(path string) bool {
	return c[path]
}

func (c chartTracker) isPathInChart(path string) bool {
	p := path
	root := fmt.Sprintf("%c", filepath.Separator)
	for p != root {
		if c[p] {
			return true
		}
		p = filepath.Dir(p)
	}
	return false
}

// looksLikeChart returns `true` if the path `dir` (assumed to be a
// directory) looks like it contains a Helm chart, rather than
// manifest files.
func looksLikeChart(dir string) bool {
	// These are the two mandatory parts of a chart. If they both
	// exist, chances are it's a chart. See
	// https://github.com/kubernetes/helm/blob/master/docs/charts.md#the-chart-file-structure
	chartpath := filepath.Join(dir, "Chart.yaml")
	valuespath := filepath.Join(dir, "values.yaml")
	if _, err := os.Stat(chartpath); err != nil && os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(valuespath); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// ParseMultidoc takes a dump of config (a multidoc YAML) and
// constructs an object set from the resources represented therein.
func ParseMultidoc(multidoc []byte, source string) (map[string]KubeManifest, error) {
	objs := map[string]KubeManifest{}
	decoder := yaml.NewDecoder(bytes.NewReader(multidoc))
	var obj KubeManifest
	var err error
	for {
		// In order to use the decoder to extract raw documents
		// from the stream, we decode generically and encode again.
		// The result is the raw document from the stream
		// (pretty-printed and without comments)
		// NOTE: gopkg.in/yaml.v3 supports round tripping comments
		//       by using `gopkg.in/yaml.v3.Node`.
		var val interface{}
		if err = decoder.Decode(&val); err != nil {
			break
		}
		bytes, err := yaml.Marshal(val)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing YAML doc from %q", source)
		}

		if obj, err = unmarshalObject(source, bytes); err != nil {
			return nil, errors.Wrapf(err, "parsing YAML doc from %q", source)
		}
		if obj == nil {
			continue
		}
		// Lists must be treated specially, since it's the
		// contained resources we are after.
		if list, ok := obj.(*List); ok {
			for _, item := range list.Items {
				objs[item.ResourceID().String()] = item
			}
		} else {
			objs[obj.ResourceID().String()] = obj
		}
	}

	if err != io.EOF {
		return objs, errors.Wrapf(err, "scanning multidoc from %q", source)
	}
	return objs, nil
}
