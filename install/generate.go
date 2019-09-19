// +build ignore

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/shurcooL/vfsgen"

	"github.com/fluxcd/flux/install"
)

func main() {
	usage := func() {
		fmt.Fprintf(os.Stderr, "usage: %s {embedded-templates,deploy}\n", os.Args[0])
		os.Exit(1)
	}
	if len(os.Args) != 2 {
		usage()
	}
	switch os.Args[1] {
	case "embedded-templates":
		var fs http.FileSystem = http.Dir("templates/")
		err := vfsgen.Generate(fs, vfsgen.Options{
			Filename:     "generated_templates.gogen.go",
			PackageName:  "install",
			VariableName: "templates",
		})
		if err != nil {
			log.Fatalln(err)
		}
	case "deploy":
		params := install.TemplateParameters{
			GitURL:    "git@github.com:fluxcd/flux-get-started",
			GitBranch: "master",
		}
		manifests, err := install.FillInTemplates(params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to fill in templates: %s\n", err)
			os.Exit(1)
		}
		for fileName, contents := range manifests {
			if err := ioutil.WriteFile(fileName, contents, 0600); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to write deploy file %s: %s\n", fileName, err)
				os.Exit(1)
			}
		}

	default:
		usage()
	}
}
