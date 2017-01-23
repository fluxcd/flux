package main

import (
	"database/sql"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/weaveworks/flux/db"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	// Parameters
	databaseSourceParam = "database-source"
	listenParam         = "listen"
	queriesParam        = "queries"
	tickParam           = "tick"
	namespaceParam      = "prometheus-namespace"
	subsystemParam      = "prometheus-subsystem"
	nameParam           = "prometheus-name"

	// Defaults
	defaultServerPort     string = ":80"
	defaultDatabaseSource string = "file://fluxy.db"
	defaultQueries        string = "queries.yaml"
	defaultTick           string = "5s"
	defaultNamespace      string = "flux"
	defaultSubsystem      string = "jobs"
	defaultName           string = "db_status_count"

	LabelName = "name"
)

// Represents the names and queries that the user wants to perform on the database
type queries struct {
	Queries []struct {
		Name  string
		Query string
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	bindLocalFlag(rootCmd, databaseSourceParam, defaultDatabaseSource, `Database source name; includes the DB driver as the scheme. The default is a temporary, file-based DB`)
	bindLocalFlag(rootCmd, listenParam, defaultServerPort, `Listen address for API clients`)
	bindLocalFlag(rootCmd, queriesParam, defaultQueries, `Yaml file with list of queries to perform`)
	bindLocalFlag(rootCmd, tickParam, defaultTick, `Query request period`)
	bindLocalFlag(rootCmd, namespaceParam, defaultNamespace, `Namespace for the prometheus variable`)
	bindLocalFlag(rootCmd, subsystemParam, defaultSubsystem, `Subsystem for the namespace variable`)
	bindLocalFlag(rootCmd, nameParam, defaultName, `Name for the prometheus variable`)
	viper.AutomaticEnv() // read in environment variables that match
}

func bindLocalFlag(c *cobra.Command, name string, value string, help string) {
	c.Flags().String(name, value, help)
	viper.BindPFlag(name, c.Flags().Lookup(name))
}

var rootCmd = &cobra.Command{
	Use:   "fluxmon",
	Short: "Monitor a database and expose metrics for prometheus",
	Long:  `This service will monitor a database for specified queries and expose them to prometheus`,
	Run: func(cmd *cobra.Command, args []string) {
		// Logger component.
		var logger log.Logger
		{
			logger = log.NewLogfmtLogger(os.Stderr)
			logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
			logger = log.NewContext(logger).With("caller", log.DefaultCaller)
		}

		// Parse queries
		queryBytes, err := ioutil.ReadFile(viper.GetString(queriesParam))
		if err != nil {
			logger.Log("stage", "read queries", "err", err)
			os.Exit(1)
		}
		var queries queries
		err = yaml.Unmarshal(queryBytes, &queries)
		if err != nil {
			logger.Log("stage", "read queries", "err", err)
			os.Exit(1)
		}
		logger.Log("stage", "read queries", "queries", fmt.Sprintf("%v", len(queries.Queries)))

		// Initialise database driver
		var dbDriver string
		{
			var version uint64
			u, err := url.Parse(viper.GetString(databaseSourceParam))
			if err != nil {
				logger.Log("stage", "db init", "err", err)
				os.Exit(1)
			}
			logger.Log("stage", "db init", "url", u, "scheme", u.Scheme)
			dbDriver = db.DriverForScheme(u.Scheme)
			logger.Log("stage", "db init", "driver", dbDriver, "db-version", fmt.Sprintf("%d", version))
		}

		// Connect to Job store.
		conn, err := sql.Open(dbDriver, viper.GetString(databaseSourceParam))
		if err != nil {
			logger.Log("stage", "db init", "err", err)
			os.Exit(1)
		}

		// Prometheus gauge
		jobStatus := prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: viper.GetString(namespaceParam),
			Subsystem: viper.GetString(subsystemParam),
			Name:      viper.GetString(nameParam),
			Help:      "Gauge for database count",
		}, []string{LabelName})

		// Error channel
		errc := make(chan error)

		// Start tick
		logger.Log("stage", "ticker", "period", viper.GetDuration(tickParam))
		dbTicker := time.NewTicker(viper.GetDuration(tickParam))
		defer dbTicker.Stop()
		go func(tick <-chan time.Time) {
			for range tick {
				for _, q := range queries.Queries {
					var count int
					err = conn.QueryRow(q.Query).Scan(&count)
					if err != nil {
						logger.Log("stage", "query", "name", q.Name, "query", q.Query, "err", err)
						errc <- err
					}
					logger.Log("stage", "query", "name", q.Name, "result", fmt.Sprintf("%v", count))
					jobStatus.With(
						LabelName, q.Name,
					).Set(float64(count))
				}
			}
		}(dbTicker.C)

		// Start prometheus metrics endpoint
		go func() {
			logger.Log("stage", "httpserver", "addr", viper.GetString(listenParam))
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			errc <- http.ListenAndServe(viper.GetString(listenParam), mux)
		}()

		logger.Log("exiting", <-errc)
	},
}
