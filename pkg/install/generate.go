// +build ignore

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shurcooL/vfsgen"

	"github.com/fluxcd/flux/pkg/install"
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
		var fs http.FileSystem = modTimeFS{
			fs: http.Dir("templates/"),
		}
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
			Namespace: "flux",
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

// modTimeFS is a wrapper that rewrites all mod times to Unix epoch.
// This is to ensure `generated_templates.gogen.go` only changes when
// the folder and/or file contents change.
type modTimeFS struct {
	fs http.FileSystem
}

func (fs modTimeFS) Open(name string) (http.File, error) {
	f, err := fs.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return modTimeFile{f}, nil
}

type modTimeFile struct {
	http.File
}

func (f modTimeFile) Stat() (os.FileInfo, error) {
	fi, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return modTimeFileInfo{fi}, nil
}

type modTimeFileInfo struct {
	os.FileInfo
}

func (modTimeFileInfo) ModTime() time.Time {
	return time.Unix(0, 0)
}
