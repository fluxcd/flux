package install

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/shurcooL/httpfs/vfsutil"
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
}

func FillInInstallTemplates(opts TemplateParameters) (io.Reader, error) {
	result := bytes.NewBuffer(nil)
	err := vfsutil.WalkFiles(templates, "/", func(path string, info os.FileInfo, rs io.ReadSeeker, err error) error {
		if err != nil {
			return fmt.Errorf("cannot walk embedded files: %s", err)
		}
		if info.IsDir() {
			return nil
		}
		templateBytes, err := ioutil.ReadAll(rs)
		if err != nil {
			return fmt.Errorf("cannot read embedded file %q: %s", info.Name(), err)
		}
		template, err := template.New(info.Name()).Parse(string(templateBytes))
		if err != nil {
			return fmt.Errorf("cannot parse embedded file %q: %s", info.Name(), err)
		}
		if err := template.Execute(result, opts); err != nil {
			return fmt.Errorf("cannot execute template for embedded file %q: %s", info.Name(), err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("internal error filling embedded installation templates: %s", err)
	}
	return result, nil
}
