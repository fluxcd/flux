---
title: Monitoring Weave Flux
menu_order: 70
---

The flux daemon exposes `/metrics` endpoints which can be scraped for
monitoring data in Prometheus format; exact metric names and help are
available from the endpoints themselves.

# flux

The following metrics are exposed:

* Duration of connection to fluxsvc
* Cluster request latencies
