package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func newTabwriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

func imageParts(image string) (string, string) {
	imageParts := strings.SplitN(image, ":", 2)
	imageName := imageParts[0]
	tag := ""
	if len(imageParts) > 1 {
		tag = imageParts[1]
	}
	return imageName, tag
}

func imageFromParts(name, tag string) string {
	return fmt.Sprintf("%s:%s", name, tag)
}
