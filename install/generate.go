// +build ignore

package main

import (
	"log"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {
	var fs http.FileSystem = http.Dir("templates/")
	err := vfsgen.Generate(fs, vfsgen.Options{
		Filename:     "generated_templates.gogen.go",
		PackageName:  "install",
		VariableName: "templates",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
