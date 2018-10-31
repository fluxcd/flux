---
title: Monitoring Weave Flux
menu_order: 70
---

The flux daemon exposes `/metrics` endpoints which can be scraped for
monitoring data in Prometheus format; exact metric names and help are
available from the endpoints themselves.

# flux

The following metrics are exposed:

| metric                                | description                             |
|---------------------------------------|-----------------------------------------|
| `flux_cache_request_duration_seconds` | Duration of cache requests, in seconds. |
| `flux_client_fetch_duration_seconds`  | Duration of remote image metadata requests |
| `flux_daemon_job_duration_seconds`    | Duration of job execution, in seconds |
| `flux_daemon_queue_duration_seconds`  | Duration of time spent in the job queue before execution |
| `flux_daemon_queue_length_count`      | Count of jobs waiting in the queue to be run |
| `flux_daemon_sync_duration_seconds`   | Duration of git-to-cluster synchronisation |
| `flux_registry_fetch_duration_seconds` | Duration of image metadata requests (from cache) |
| `flux_fluxd_connection_duration_seconds` | Duration in seconds of the current connection to fluxsvc |
