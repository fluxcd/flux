---
title: Monitoring Weave Flux
menu_order: 40
---

Both fluxd and fluxsvc expose `/metrics` endpoints which can be
scraped for monitoring data in Prometheus format; exact metric names
and help are available from the endpoints themselves.

# fluxd

The following metrics are exposed:

* Duration of connection to fluxsvc
* Platform request latencies

# fluxsvc

When using Weave Cloud you will be connected to Weaveworks' hosted
multi-tenant fluxsvc, so metrics won't be available to you. If you
have deployed fluxsvc yourself as part of a standalone deployment, the
following metrics are exposed:

* Number of connected daemons
* API request latencies
