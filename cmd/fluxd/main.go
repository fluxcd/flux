package main

import (
	"fmt"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"

	"github.com/weaveworks/fluxy/api"
)

func main() {
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxd is a deployment daemon.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		pflag.PrintDefaults()
	}
	var (
		listenAddr = pflag.StringP("listen", "l", ":3030", "Listen address for Flux API clients")
		// credsPath = pflag.StringP("credentials", "", "", "Path to a credentials file à la Docker, e.g. mounted as a secret")
	)
	pflag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	s := api.Server{}
	logger.Log("listening", *listenAddr)
	logger.Log("err", s.ListenAndServe(*listenAddr))
}
