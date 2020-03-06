# Monitoring Flux

The Flux daemon exposes `/metrics` endpoints which can be scraped for
monitoring data in Prometheus format; exact metric names and help are
available from the endpoints themselves.

The following metrics are exposed:

| metric                                   | description
| ---------------------------------------- | ---
| `flux_cache_request_duration_seconds`    | Duration of cache requests, in seconds.
| `flux_client_fetch_duration_seconds`     | Duration of remote image metadata requests
| `flux_daemon_job_duration_seconds`       | Duration of job execution, in seconds
| `flux_daemon_queue_duration_seconds`     | Duration of time spent in the job queue before execution
| `flux_daemon_queue_length_count`         | Count of jobs waiting in the queue to be run
| `flux_daemon_sync_duration_seconds`      | Duration of git-to-cluster synchronisation
| `flux_daemon_sync_manifests`             | Number of manifests being synced to cluster
| `flux_registry_fetch_duration_seconds`   | Duration of image metadata requests (from cache)
| `flux_fluxd_connection_duration_seconds` | Duration in seconds of the current connection to fluxsvc

Flux sync state can be obtained by using the following PromQL expressions:
* `delta(flux_daemon_sync_duration_seconds_count{success='true'}[6m]) < 1` - for general flux sync errors - usually if 
that is true then there are some problems with infrastructure or there are manifests parse error or there are manifests 
with duplicate ids.

* `flux_daemon_sync_manifests{success='false'} > 0` - for git manifests errors - if true then there are either some 
problems with applying git manifests to kubernetes - e.g. configmap size is too big to fit in annotations or 
immutable field (like label selector) was changed. 
