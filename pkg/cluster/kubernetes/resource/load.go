package resource

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.mozilla.org/sops/v3"
	"go.mozilla.org/sops/v3/decrypt"
	"gopkg.in/yaml.v2"
)

// Load takes paths to directories or files, and creates an object set
// based on the file(s) therein. Resources are named according to the
// file content, rather than the file name of directory structure. if
// sopsEnabled is set to true, sops-encrypted files will be decrypted.
func Load(base string, paths []string, sopsEnabled bool) (map[string]KubeManifest, error) {
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, fmt.Errorf("git path %q not found", base)
	}
	objs := map[string]KubeManifest{}
	charts, err := newChartTracker(base)
	if err != nil {
		return nil, errors.Wrapf(err, "walking %q for chartdirs", base)
	}
	for _, root := range paths {
		// In the walk, we ignore errors (indicating a failure to read
		// a file) if it's not a file of interest. However, we _are_
		// interested in the error if an explicitly-mentioned path
		// does not exist.
		if _, err := os.Stat(root); err != nil {
			return nil, errors.Wrapf(err, "unable to read root path %q", root)
		}
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				if charts.isDirChart(path) {
					return filepath.SkipDir
				}
				return nil
			}

			// No need to check for errors for files we are not interested in anyway.
			if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
				return nil
			}
			if charts.isPathInChart(path) {
				return nil
			}

			if err != nil {
				return errors.Wrapf(err, "walking file %q for yaml docs", path)
			}

			// Load file
			bytes, err := loadFile(path, sopsEnabled)
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
			return nil
		})
		if err != nil {
			return objs, err
		}
	}

	return objs, nil
}

// chartTracker keeps track of paths that contain Helm charts in them.
type chartTracker map[string]bool

func newChartTracker(root string) (chartTracker, error) {
	chartdirs := make(chartTracker)
	// Enumerate directories that contain charts. This will never
	// return an error since our callback function swallows it.
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If a file or directory cannot be walked now we presume it will
			// also not be available for walking when looking for yamels. If
			// we do need access to it we can raise the error there.
			return nil
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
	return chartdirs, nil
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
				id := item.ResourceID().String()
				if _, ok := objs[id]; ok {
					return nil, fmt.Errorf(`duplicate definition of '%s' (in %s)`, id, source)
				}
				objs[id] = item
			}
		} else {
			id := obj.ResourceID().String()
			if _, ok := objs[id]; ok {
				return nil, fmt.Errorf(`duplicate definition of '%s' (in %s)`, id, source)
			}
			objs[id] = obj
		}
	}

	if err != io.EOF {
		return objs, errors.Wrapf(err, "scanning multidoc from %q", source)
	}
	return objs, nil
}

// loadFile attempts to load a file from the path supplied. If sopsEnabled is set,
// it will try to decrypt it before returning the data
func loadFile(path string, sopsEnabled bool) ([]byte, error) {
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if sopsEnabled && bytes.Contains(fileBytes, []byte("sops:")) {
		return softDecrypt(fileBytes)
	}
	return fileBytes, nil
}

// softDecrypt takes data from a file and tries to decrypt it with sops,
// if the file has not been encrypted with sops, the original data will be returned
func softDecrypt(rawData []byte) ([]byte, error) {
	decryptedData, err := decrypt.Data(rawData, "yaml")
	if err == sops.MetadataNotFound {
		return rawData, nil
	} else if err != nil {
		return rawData, errors.Wrap(err, "failed to decrypt file")
	}
	return decryptedData, nil
}
