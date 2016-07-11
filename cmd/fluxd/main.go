package main

import (
	"fmt"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/api"
)

func main() {
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}
	opts := &daemonOpts{
		logger: logger,
	}
	cmd := &cobra.Command{
		Use:   "fluxd",
		Short: "the flux deployment daemon",
		RunE:  opts.run,
	}
	opts.addFlags(cmd)
	cmd.Execute()
}

type daemonOpts struct {
	logger     log.Logger
	listenAddr string
	credsPath  string
}

func (opts *daemonOpts) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&opts.listenAddr, "listen", "l", ":3030", "the address to listen for flux API clients on")
	cmd.Flags().StringVar(&opts.credsPath, "credentials", "", "path to a credentials file a la Docker, e.g., mounted as a secret")
}

func (opts *daemonOpts) run(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("did not expect any arguments")
	}
	apisrv := api.APIServer{}
	opts.logger.Log("listen-err", apisrv.ListenAndServe(opts.listenAddr))
	return nil
}
