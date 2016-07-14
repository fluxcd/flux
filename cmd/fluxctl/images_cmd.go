package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/registry"
)

type imagesOpts struct {
	*rootOpts
}

func imagesCommand(rootOpts *rootOpts) *cobra.Command {
	opts := &imagesOpts{rootOpts: rootOpts}
	cmd := &cobra.Command{
		Use:   "images <repository>",
		Short: "list images available for an image repository",
		RunE:  opts.RunE,
	}
	return cmd
}

func (opts *imagesOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf(`expected argument <repository>, e.g., "quay.io/weaveworks/helloworld`)
	}

	resp, err := http.Get(opts.URL + "/images/" + args[0])
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(`expected response "%d OK" from API server; got "%s"`,
			http.StatusOK, resp.Status)
	}

	var repository registry.Repository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return err
	}

	out := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
	images := repository.Images
	sort.Sort(registry.ImagesByCreatedDesc{images})
	fmt.Fprintln(out, "IMAGE\tCREATED")
	for _, image := range images {
		fmt.Fprintf(out, "%s:%s\t%s\n", image.Name, image.Tag, image.CreatedAt)
	}
	out.Flush()
	return nil
}
