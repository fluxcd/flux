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

	"github.com/weaveworks/flux/automator"
	"github.com/weaveworks/flux/db"
	"github.com/weaveworks/flux/history"
	historysql "github.com/weaveworks/flux/history/sql"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/instance"
	instancedb "github.com/weaveworks/flux/instance/sql"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/rpc/nats"
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
		natsURL               = fs.String("nats-url", "", `URL on which to connect to NATS, or empty to use the standalone message bus (e.g., "nats://user:pass@nats:4222")`)
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
		busMetrics     platform.BusMetrics
	)
	{
		httpDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"method", "status_code"})
		serverMetrics.ListServicesDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "list_services_duration_seconds",
			Help:      "ListServices method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"namespace", "success"})
		serverMetrics.ListImagesDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "list_images_duration_seconds",
			Help:      "ListImages method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"service_spec", "success"})
		serverMetrics.HistoryDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "history_duration_seconds",
			Help:      "History method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"service_spec", "success"})
		serverMetrics.RegisterDaemonDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "register_daemon_duration_seconds",
			Help:      "RegisterDaemon method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"instance_id", "success"})
		serverMetrics.ConnectedDaemons = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "connected_daemons_count",
			Help:      "Gauge of the current number of connected daemons",
		}, []string{})
		serverMetrics.PlatformMetrics = platform.NewMetrics()
		releaseMetrics.ReleaseDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_duration_seconds",
			Help:      "Release method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"release_type", "release_kind", "success"})
		releaseMetrics.ActionDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_action_duration_seconds",
			Help:      "Duration in seconds of each sub-action invoked as part of a non-dry-run release.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"action", "success"})
		releaseMetrics.StageDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_stage_duration_seconds",
			Help:      "Duration in seconds of each stage of a release, including dry-runs.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"method", "stage"})
		helperDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "fluxsvc",
			Name:      "release_helper_duration_seconds",
			Help:      "Duration in seconds of a variety of release helper methods.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"method", "success"})
		busMetrics = platform.NewBusMetrics()
	}

	var messageBus platform.MessageBus
	{
		if *natsURL != "" {
			bus, err := nats.NewMessageBus(*natsURL, busMetrics)
			if err != nil {
				logger.Log("component", "message bus", "err", err)
				os.Exit(1)
			}
			logger.Log("component", "message bus", "type", "NATS")
			messageBus = bus
		} else {
			messageBus = platform.NewStandaloneMessageBus(busMetrics)
			logger.Log("component", "message bus", "type", "standalone")
		}
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

	// Job store.
	var jobStore jobs.JobStore
	{
		s, err := jobs.NewDatabaseStore(dbDriver, *databaseSource, time.Hour)
		if err != nil {
			logger.Log("component", "release job store", "err", err)
			os.Exit(1)
		}
		jobStore = s
	}

	// Automator component.
	var auto *automator.Automator
	{
		var err error
		auto, err = automator.New(automator.Config{
			Jobs:       jobStore,
			InstanceDB: instanceDB,
			Instancer:  instancer,
			Logger:     log.NewContext(logger).With("component", "automator"),
		})
		if err == nil {
			logger.Log("automator", "enabled")
		} else {
			// Service can handle a nil automator pointer.
			logger.Log("automator", "disabled", "reason", err)
		}
	}

	go auto.Start(log.NewContext(logger).With("component", "automator"))

	// Job workers.
	{
		logger := log.NewContext(logger).With("component", "worker")
		worker := jobs.NewWorker(jobStore, logger)
		worker.Register(jobs.AutomatedServiceJob, auto)
		worker.Register(jobs.AutomatedInstanceJob, auto)
		worker.Register(jobs.ReleaseJob, release.NewReleaser(instancer, releaseMetrics))

		defer func() {
			if err := worker.Stop(shutdownTimeout); err != nil {
				logger.Log("err", err)
			}
		}()
		go worker.Work()

		cleaner := jobs.NewCleaner(jobStore, logger)
		cleanTicker := time.NewTicker(15 * time.Second)
		defer cleanTicker.Stop()
		go cleaner.Clean(cleanTicker.C)
	}

	// The server.
	server := server.New(instancer, messageBus, jobStore, logger, serverMetrics)

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
