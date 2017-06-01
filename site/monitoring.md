---
title: Monitoring Weave Flux
menu_order: 70
---

Both fluxd and fluxsvc expose `/metrics` endpoints which can be
scraped for monitoring data in Prometheus format; exact metric names
and help are available from the endpoints themselves.

# fluxd

The following metrics are exposed:

* Duration of connection to fluxsvc
* Platform request latencies
