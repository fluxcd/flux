package resource

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/resource"
)

// Load takes paths to directories or files, and creates an object set
// based on the file(s) therein. Resources are named according to the
// file content, rather than the file name of directory structure.
func Load(roots ...string) (map[string]resource.Resource, error) {
	objs := map[string]resource.Resource{}
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.Wrapf(err, "walking %q for yamels", path)
			}
			if !info.IsDir() && filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
				bytes, err := ioutil.ReadFile(path)
				if err != nil {
					return errors.Wrapf(err, "reading file at %q", path)
				}
				docsInFile, err := ParseMultidoc(bytes, path)
				if err != nil {
					return errors.Wrapf(err, "parsing file at %q", path)
				}
				for id, obj := range docsInFile {
					if alreadyDefined, ok := objs[id]; ok {
						return fmt.Errorf(`resource '%s' defined more than once (in %s and %s)`, id, alreadyDefined.Source(), path)
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

// ParseMultidoc takes a dump of config (a multidoc YAML) and
// constructs an object set from the resources represented therein.
func ParseMultidoc(multidoc []byte, source string) (map[string]resource.Resource, error) {
	objs := map[string]resource.Resource{}
	chunks := bufio.NewScanner(bytes.NewReader(multidoc))
	initialBuffer := make([]byte, 4096)     // Matches startBufSize in bufio/scan.go
	chunks.Buffer(initialBuffer, 1024*1024) // Allow growth to 1MB
	chunks.Split(splitYAMLDocument)

	var obj *BaseObject
	var err error
	for chunks.Scan() {
		// It's not guaranteed that the return value of Bytes() will not be mutated later:
		// https://golang.org/pkg/bufio/#Scanner.Bytes
		// But we will be snaffling it away, so make a copy.
		bytes := chunks.Bytes()
		bytes2 := make([]byte, len(bytes), cap(bytes))
		copy(bytes2, bytes)
		if obj, err = unmarshalObject(source, bytes2); err != nil {
			return nil, errors.Wrapf(err, "parsing YAML doc from %q", source)
		}
		if obj == nil {
			continue
		}

		if obj.Kind == "List" {
			err := unmarshalList(source, obj, objs)

			if err != nil {
				return nil, err
			}
		} else {
			r, err := unmarshalKind(*obj, obj.Bytes())

			if r == nil {
				continue
			}

			// Catch if a resource is defined in a List AND in a file
			if resourceAlreadyExists(objs, r) {
				return nil, cluster.ErrMultipleResourceDefinitionsFoundForService
			}

			if err != nil {
				return nil, err
			}

			objs[obj.ResourceID().String()] = r
		}
	}

	if err := chunks.Err(); err != nil {
		return objs, errors.Wrapf(err, "scanning multidoc from %q", source)
	}

	return objs, nil
}

func resourceAlreadyExists(existing map[string]resource.Resource, next resource.Resource) bool {
	_, ok := existing[next.ResourceID().String()]
	return ok
}

// ---
// Taken directly from https://github.com/kubernetes/apimachinery/blob/master/pkg/util/yaml/decoder.go.

const yamlSeparator = "\n---"

// splitYAMLDocument is a bufio.SplitFunc for splitting YAML streams into individual documents.
func splitYAMLDocument(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	sep := len([]byte(yamlSeparator))
	if i := bytes.Index(data, []byte(yamlSeparator)); i >= 0 {
		// We have a potential document terminator
		i += sep
		after := data[i:]
		if len(after) == 0 {
			// we can't read any more characters
			if atEOF {
				return len(data), data[:len(data)-sep], nil
			}
			return 0, nil, nil
		}
		if j := bytes.IndexByte(after, '\n'); j >= 0 {
			return i + j + 1, data[0 : i-sep], nil
		}
		return 0, nil, nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

// ---
