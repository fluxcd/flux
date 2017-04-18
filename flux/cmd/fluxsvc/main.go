package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"

	"github.com/weaveworks/fluxy/flux/automator"
	"github.com/weaveworks/fluxy/flux/db"
	"github.com/weaveworks/fluxy/flux/fluxsvc"
	"github.com/weaveworks/fluxy/flux/history"
	"github.com/weaveworks/fluxy/flux/orgmap"
	"github.com/weaveworks/fluxy/flux/registry"
	"github.com/weaveworks/fluxy/flux/release"
)

func main() {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxsvc is part of the flux change management tool.\n")
		fmt.Fprintf(os.Stderr, "  It is designed to run separately from your fluxd instance.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	// This mirrors how kubectl extracts information from the environment.
	var (
		listenAddr     = fs.StringP("listen", "l", ":3030", "Listen address for fluxctl clients")
		databaseDriver = fs.String("database-driver", "ql-mem", `Database driver name, e.g., "postgres"; the default is an in-memory DB`)
		databaseSource = fs.String("database-source", "history.db", `Database source name; specific to the database driver (--database-driver) used. The default is an arbitrary, in-memory DB name`)
		//repoURL         = fs.String("repo-url", "", "Config repo URL, e.g. git@github.com:myorg/conf (required)")
		//repoKey         = fs.String("repo-key", "", "SSH key file with commit rights to config repo")
		//repoPath        = fs.String("repo-path", "", "Path within config repo to look for resource definition files")
		//slackWebhookURL = fs.String("slack-webhook-url", "", "Slack webhook URL for release notifications (optional)")
		//slackUsername   = fs.String("slack-username", "fluxy-deploy", "Slack username for release notifications")
	)
	fs.Parse(os.Args)

	// Logging.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	// Instrumentation.
	var (
		httpDuration metrics.Histogram
		metrics      fluxsvc.Metrics
	)
	{
		httpDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
		}, []string{"method", "status_code"})
		metrics.ListServicesDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "list_services_duration_seconds",
			Help:      "ListServices duration in seconds.",
		}, []string{"namespace", "success"})
		metrics.ListImagesDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "list_images_duration_seconds",
			Help:      "ListImages duration in seconds.",
		}, []string{"service_spec", "success"})
		metrics.ReleaseDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_duration_seconds",
			Help:      "Release duration in seconds.",
		}, []string{"kind", "success"})
		metrics.AutomateDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "automate_duration_seconds",
			Help:      "Automate duration in seconds.",
		}, []string{"success"})
		metrics.DeautomateDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "deautomate_duration_seconds",
			Help:      "Deautomate duration in seconds.",
		}, []string{"success"})
		metrics.HistoryDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "history_duration_seconds",
			Help:      "History duration in seconds.",
		}, []string{"success"})
	}

	// Organization mapper.
	var mapper orgmap.Mapper
	{
		// TODO(pb)
	}

	// Image repository.
	var imageRepo *registry.Repository
	{
		// TODO(pb)
	}

	// Automator.
	var automator automator.Automator
	{
		// TODO(pb)
	}

	// Releaser, i.e. release job read/writer.
	var releaser release.JobReadWriter
	{
		// TODO(pb)
	}

	var eventReader history.EventReader
	{
		db, err := db.NewHistoryDB(*databaseDriver, *databaseSource)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		eventReader = db
	}

	// Service.
	var service fluxsvc.Service
	{
		service = fluxsvc.NewServer(
			mapper,
			imageRepo,
			automator,
			releaser,
			eventReader,
			logger,
			metrics,
		)
	}

	// Mechanical stuff.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// HTTP transport component.
	go func() {
		logger.Log("addr", *listenAddr)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.Handle("/", fluxsvc.NewHandler(service, fluxsvc.NewRouter(), logger, httpDuration))
		errc <- http.ListenAndServe(*listenAddr, mux)
	}()

	// Go!
	logger.Log("exit", <-errc)

}
