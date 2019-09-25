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

type TemplateParameters struct {
	GitURL             string
	GitBranch          string
	GitPaths           []string
	GitLabel           string
	GitUser            string
	GitEmail           string
	Namespace          string
	AdditionalFluxArgs []string
	ConfigFile         string
	ConfigFileContent  string
	ConfigFileReader   io.Reader
	ConfigAsConfigMap  bool
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.Replace(v, "\n", "\n"+pad, -1)
}

func nindent(spaces int, v string) string {
	return "\n" + indent(spaces, v)
}

func FillInTemplates(params TemplateParameters) (map[string][]byte, error) {
	result := map[string][]byte{}

	if params.ConfigFileReader != nil {
		tmp, err := ioutil.ReadAll(params.ConfigFileReader)
		if err != nil {
			return map[string][]byte{}, fmt.Errorf("unable to read config file: %s", err)
		}
		var f chartutil.Files
		f = make(map[string][]byte)
		f["flux-config.yaml"] = tmp
		if !params.ConfigAsConfigMap {
			params.ConfigFileContent = f.AsSecrets()
		} else {
			params.ConfigFileContent = f.AsConfig()
		}
	}

	err := vfsutil.WalkFiles(templates, "/", func(path string, info os.FileInfo, rs io.ReadSeeker, err error) error {
		if err != nil {
			return fmt.Errorf("cannot walk embedded files: %s", err)
		}
		if info.IsDir() {
			return nil
		}
		manifestTemplateBytes, err := ioutil.ReadAll(rs)
		if err != nil {
			return fmt.Errorf("cannot read embedded file %q: %s", info.Name(), err)
		}
		manifestTemplate, err := template.New(info.Name()).
			Funcs(template.FuncMap{"StringsJoin": strings.Join, "nindent": nindent}).
			Parse(string(manifestTemplateBytes))
		if err != nil {
			return fmt.Errorf("cannot parse embedded file %q: %s", info.Name(), err)
		}
		out := bytes.NewBuffer(nil)
		if err := manifestTemplate.Execute(out, params); err != nil {
			return fmt.Errorf("cannot execute template for embedded file %q: %s", info.Name(), err)
		}
		result[strings.TrimSuffix(info.Name(), ".tmpl")] = out.Bytes()
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("internal error filling embedded installation templates: %s", err)
	}
	return result, nil
}
