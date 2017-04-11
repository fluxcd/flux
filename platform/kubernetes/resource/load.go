package resource

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Load takes a path to a directory or file, and creates an object set
// based on the file(s) therein. Resources are named according to the
// file content, rather than the file name of directory structure.
func Load(root string) (map[string]Resource, error) {
	objs := map[string]Resource{}
	var err error
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf(`walking %q for yamels: %s`, path, err.Error())
		}
		if !info.IsDir() && filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
			bytes, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf(`reading file at "%s": %s`, path, err.Error())
			}
			docsInFile, err := ParseMultidoc(bytes, path)
			if err != nil {
				return fmt.Errorf(`parsing file at "%s": %s`, path, err.Error())
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
	return objs, err
}

// ParseManifests takes a dump of config (a multidoc YAML) and
// constructs an object set from the resources represented therein.
func ParseMultidoc(multidoc []byte, source string) (map[string]Resource, error) {
	objs := map[string]Resource{}
	chunks := bufio.NewScanner(bytes.NewReader(multidoc))
	chunks.Split(splitYAMLDocument)

	for chunks.Scan() {
		if obj, err := unmarshalObject(source, chunks.Bytes()); err != nil {
			return nil, fmt.Errorf(`parsing YAML doc from "%s": %s`, source, err.Error())
		} else {
			objs[obj.ResourceID()] = obj
		}
	}
	if err := chunks.Err(); err != nil {
		return objs, fmt.Errorf(`scanning multidoc from "%s": %s`, source, err.Error())
	}
	return objs, nil
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
