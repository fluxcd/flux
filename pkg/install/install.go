package install

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/shurcooL/httpfs/vfsutil"
	"k8s.io/helm/pkg/chartutil"
)

//go:generate go run generate.go

type TemplateParameters struct {
	GitURL                  string
	GitBranch               string
	GitPaths                []string
	GitLabel                string
	GitUser                 string
	GitEmail                string
	GitReadOnly             bool
	RegistryDisableScanning bool
	AddSecurityContext      bool
	Namespace               string
	ManifestGeneration      bool
	AdditionalFluxArgs      []string
	ConfigFileContent       string
	ConfigAsConfigMap       bool
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.Replace(v, "\n", "\n"+pad, -1)
}

func FillInTemplates(params TemplateParameters) (map[string][]byte, error) {
	result := map[string][]byte{}

	err := vfsutil.WalkFiles(templates, "/", func(path string, info os.FileInfo, rs io.ReadSeeker, err error) error {
		if err != nil {
			return fmt.Errorf("cannot walk embedded files: %s", err)
		}
		if info.IsDir() {
			return nil
		}
		if params.RegistryDisableScanning && strings.Contains(info.Name(), "memcache") {
			// do not include memcached resources when registry scanning is disabled
			return nil
		}
		manifestTemplateBytes, err := ioutil.ReadAll(rs)
		if err != nil {
			return fmt.Errorf("cannot read embedded file %q: %s", info.Name(), err)
		}
		manifestTemplate, err := template.New(info.Name()).
			Funcs(template.FuncMap{"StringsJoin": strings.Join, "indent": indent}).
			Parse(string(manifestTemplateBytes))
		if err != nil {
			return fmt.Errorf("cannot parse embedded file %q: %s", info.Name(), err)
		}
		out := bytes.NewBuffer(nil)
		if err := manifestTemplate.Execute(out, params); err != nil {
			return fmt.Errorf("cannot execute template for embedded file %q: %s", info.Name(), err)
		}
		if out.Len() > 0 {
			result[strings.TrimSuffix(info.Name(), ".tmpl")] = out.Bytes()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("internal error filling embedded installation templates: %s", err)
	}
	return result, nil
}

func ConfigContent(configFile io.Reader, configAsConfigMap bool) (string, error) {
	tmp, err := ioutil.ReadAll(configFile)
	if err != nil {
		return "", fmt.Errorf("unable to read config file: %s", err)
	}
	var f chartutil.Files
	f = make(map[string][]byte)
	f["flux-config.yaml"] = tmp
	if configAsConfigMap {
		return f.AsConfig(), nil
	} else {
		return f.AsSecrets(), nil
	}
}
