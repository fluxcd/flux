---
title: Monitoring Weave Flux
menu_order: 70
---

The flux daemon exposes `/metrics` endpoints which can be scraped for
monitoring data in Prometheus format; exact metric names and help are
available from the endpoints themselves.

# flux

The following metrics are exposed:

* Duration of cache requests
* Duration of remote image metadata requests
* Duration of job execution, in seconds
* Duration of time spent in the job queue before execution
* Count of jobs waiting in the queue to be run
* Duration of git-to-cluster synchronisation
* Duration of image metadata requests (from cache)
* Duration in seconds of the current connection to fluxsvc
