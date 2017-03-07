package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

type saveOpts struct {
	*rootOpts
	output string
}

func newSave(parent *rootOpts) *saveOpts {
	return &saveOpts{rootOpts: parent}
}

func (opts *saveOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save",
		Short: "save service definitions to local files in platform-native format",
		Example: makeExample(
			"fluxctl save",
		),
		RunE: opts.RunE,
	}
	return cmd
}

// Deliberately omit fields (e.g. status, metadata.uid) that we don't want to save
type saveObject struct {
	APIVersion string `yaml:"apiVersion,omitempty"`
	Kind       string `yaml:"kind,omitempty"`

	Metadata struct {
		Annotations map[string]string `yaml:"annotations,omitempty"`
		Labels      map[string]string `yaml:"labels,omitempty"`
		Name        string            `yaml:"name,omitempty"`
		Namespace   string            `yaml:"namespace,omitempty"`
	} `yaml:"metadata,omitempty"`

	Spec map[interface{}]interface{} `yaml:"spec,omitempty"`
}

func (opts *saveOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	config, err := opts.API.Export(noInstanceID)
	if err != nil {
		return errors.Wrap(err, "exporting config")
	}

	yamls := bufio.NewScanner(bytes.NewReader(config))
	yamls.Split(splitYAMLDocument)

	for yamls.Scan() {
		var object saveObject
		// Most unwanted fields are ignored at this point
		if err := yaml.Unmarshal(yamls.Bytes(), &object); err != nil {
			return errors.Wrap(err, "unmarshalling exported yaml")
		}

		// Filter out remaining unwanted keys from unstructured fields
		// e.g. .Spec and .Metadata.Annotations
		filterObject(object)

		if err := saveYAML(object); err != nil {
			return errors.Wrap(err, "saving yaml object")
		}
	}

	if yamls.Err() != nil {
		return errors.Wrap(yamls.Err(), "splitting exported yaml")
	}

	return nil
}

// Remove any data that should not be version controlled
func filterObject(object saveObject) {
	delete(object.Metadata.Annotations, "deployment.kubernetes.io/revision")
	delete(object.Metadata.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	deleteNested(object.Spec, "template", "metadata", "creationTimestamp")
	deleteEmptyMapValues(object.Spec)
}

// Recurse through nested maps to remove a key
func deleteNested(m map[interface{}]interface{}, keys ...string) {
	switch len(keys) {
	case 0:
		return
	case 1:
		delete(m, keys[0])
	default:
		if v, ok := m[keys[0]].(map[interface{}]interface{}); ok {
			deleteNested(v, keys[1:]...)
		}
	}
}

// Recursively delete map keys with empty values
func deleteEmptyMapValues(i interface{}) bool {
	switch i := i.(type) {
	case map[interface{}]interface{}:
		if len(i) == 0 {
			return true
		} else {
			for k, v := range i {
				if deleteEmptyMapValues(v) {
					delete(i, k)
				}
			}
		}
	case []interface{}:
		if len(i) == 0 {
			return true
		} else {
			for _, e := range i {
				deleteEmptyMapValues(e)
			}
		}
	case nil:
		return true
	}
	return false
}

// Save YAML to directory structure
func saveYAML(object saveObject) error {
	var path string
	if object.Kind == "Namespace" {
		path = fmt.Sprintf("%s-ns.yaml", object.Metadata.Name)
	} else {
		dir := object.Metadata.Namespace
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return errors.Wrap(err, "making directory for namespace")
		}

		shortKind := abbreviateKind(object.Kind)
		path = fmt.Sprintf("%s/%s-%s.yaml", dir, object.Metadata.Name, shortKind)
	}

	fmt.Printf("Saving %s '%s' to %s\n", object.Kind, object.Metadata.Name, path)

	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "creating yaml file")
	}
	defer file.Close()

	buf, err := yaml.Marshal(object)
	if err != nil {
		return errors.Wrap(err, "marshalling yaml")
	}

	if _, err := file.Write(buf); err != nil {
		return errors.Wrap(err, "writing yaml file")
	}

	return nil
}

func abbreviateKind(kind string) string {
	switch kind {
	case "Service":
		return "svc"
	case "ReplicationController":
		return "rc"
	case "Deployment":
		return "dep"
	default:
		return kind
	}
}

// Copied from k8s.io/client-go/1.5/pkg/util/yaml/decoder.go

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
