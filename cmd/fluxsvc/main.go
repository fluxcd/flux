package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/automator"
	"github.com/weaveworks/flux/db"
	"github.com/weaveworks/flux/history"
	historysql "github.com/weaveworks/flux/history/sql"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/instance"
	instancedb "github.com/weaveworks/flux/instance/sql"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/release"
	"github.com/weaveworks/flux/server"
)

const shutdownTimeout = 30 * time.Second

func main() {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxsvc is a deployment service.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}

	var (
		listenAddr            = fs.StringP("listen", "l", ":3030", "Listen address for Flux API clients")
		databaseSource        = fs.String("database-source", "file://fluxy.db", `Database source name; includes the DB driver as the scheme. The default is a temporary, file-based DB`)
		databaseMigrationsDir = fs.String("database-migrations", "./db/migrations", "Path to database migration scripts, which are in subdirectories named for each driver")
	)
	fs.Parse(os.Args)

	// Logger component.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	// Initialise database; we must fail if we can't do this, because
	// most things depend on it.
	var dbDriver string
	{
		var version uint64
		u, err := url.Parse(*databaseSource)
		if err == nil {
			version, err = db.Migrate(*databaseSource, *databaseMigrationsDir)
		}

		if err != nil {
			logger.Log("stage", "db init", "err", err)
			os.Exit(1)
		}
		dbDriver = db.DriverForScheme(u.Scheme)
		logger.Log("migrations", "success", "driver", dbDriver, "db-version", fmt.Sprintf("%d", version))
	}

	// Instrumentation
	var (
		httpDuration   metrics.Histogram
		serverMetrics  server.Metrics
		releaseMetrics release.Metrics
		helperDuration metrics.Histogram
	)
	{
		httpDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
		}, []string{"method", "status_code"})
		serverMetrics.ListServicesDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "list_services_duration_seconds",
			Help:      "ListServices method duration in seconds.",
		}, []string{"namespace", "success"})
		serverMetrics.ListImagesDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "list_images_duration_seconds",
			Help:      "ListImages method duration in seconds.",
		}, []string{"service_spec", "success"})
		serverMetrics.HistoryDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "history_duration_seconds",
			Help:      "History method duration in seconds.",
		}, []string{"service_spec", "success"})
		releaseMetrics.ReleaseDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_duration_seconds",
			Help:      "Release method duration in seconds.",
		}, []string{"release_type", "release_kind", "success"})
		releaseMetrics.ActionDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_action_duration_seconds",
			Help:      "Duration in seconds of each sub-action invoked as part of a non-dry-run release.",
		}, []string{"action", "success"})
		releaseMetrics.StageDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_stage_duration_seconds",
			Help:      "Duration in seconds of each stage of a release, including dry-runs.",
		}, []string{"method", "stage"})
		helperDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_helper_duration_seconds",
			Help:      "Duration in seconds of a variety of release helper methods.",
		}, []string{"method", "success"})
	}

	var messageBus platform.MessageBus
	{
		messageBus = platform.NewStandaloneMessageBus()
	}

	var historyDB history.DB
	{
		db, err := historysql.NewSQL(dbDriver, *databaseSource)
		if err != nil {
			logger.Log("component", "history", "err", err)
			os.Exit(1)
		}
		historyDB = db
	}

	// Configuration, i.e., whether services are automated or not.
	var instanceDB instance.DB
	{
		db, err := instancedb.New(dbDriver, *databaseSource)
		if err != nil {
			logger.Log("component", "config", "err", err)
			os.Exit(1)
		}
		instanceDB = db
	}

	var instancer instance.Instancer
	{
		// Instancer, for the instancing of operations
		instancer = &instance.MultitenantInstancer{
			DB:        instanceDB,
			Connecter: messageBus,
			Logger:    logger,
			Histogram: helperDuration,
			History:   historyDB,
		}
	}

	// Release job store.
	var rjs flux.JobStore
	{
		s, err := jobs.NewDatabaseStore(dbDriver, *databaseSource, time.Hour)
		if err != nil {
			logger.Log("component", "release job store", "err", err)
			os.Exit(1)
		}
		rjs = s
	}

	// Release workers.
	{
		worker := jobs.NewWorker(rjs, instancer, releaseMetrics, logger)
		defer func() {
			if err := worker.Stop(shutdownTimeout); err != nil {
				logger.Log("err", err)
			}
		}()
		worker.Work()

		cleaner := jobs.NewCleaner(rjs, logger)
		cleanTicker := time.NewTicker(15 * time.Second)
		defer cleanTicker.Stop()
		go cleaner.Clean(cleanTicker.C)
	}

	// Automator component.
	var auto *automator.Automator
	{
		var err error
		auto, err = automator.New(automator.Config{
			Releaser:   rjs,
			InstanceDB: instanceDB,
		})
		if err == nil {
			logger.Log("automator", "enabled")
		} else {
			// Service can handle a nil automator pointer.
			logger.Log("automator", "disabled", "reason", err)
		}
	}

	go auto.Start(log.NewContext(logger).With("component", "automator"))

	// The server.
	server := server.New(instancer, messageBus, rjs, logger, serverMetrics)

	// Mechanical components.
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
		mux.Handle("/", transport.NewHandler(server, transport.NewRouter(), logger, httpDuration))
		errc <- http.ListenAndServe(*listenAddr, mux)
	}()

	logger.Log("exiting", <-errc)
}
