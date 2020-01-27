// +build tools

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shurcooL/vfsgen"
)

func main() {
	usage := func() {
		fmt.Fprintf(os.Stderr, "usage: %s\n", os.Args[0])
		os.Exit(1)
	}
	if len(os.Args) != 1 {
		usage()
	}

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
