# Scenario 05 — SPIRE Telemetry & Metrics

> **Complexity:** Advanced · **Previous:** [04-svid-api](../04-svid-api/README.md)

## What You Will Learn

SPIRE Servers and Agents can emit telemetry to track activities such as Server and Agent startup time, registration entry creation, X.509 SVID creation, and successful authentication. This scenario demonstrates how to configure the SPIRE Server and the SPIRE Agent to emit telemetry to supported metrics collectors. Two collectors are shown — **StatsD** and **Prometheus** — receiving metrics from a SPIRE deployment. Metrics collected by StatsD are displayed using **Graphite**.

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Telemetry** | Operational data (counters, gauges, timers) emitted by SPIRE components to external systems for monitoring and alerting. |
| **Prometheus** | A pull-based metrics system. SPIRE exposes an HTTP endpoint that Prometheus scrapes at regular intervals. |
| **StatsD** | A push-based metrics protocol. SPIRE pushes metrics to a StatsD-compatible collector. |
| **Graphite** | A time-series database and dashboard that stores and displays metrics pushed via StatsD. |

In this tutorial you will learn how to:
* Configure SPIRE Server and SPIRE Agent to emit telemetry
* Configure StatsD as a metrics collector
* Configure Prometheus as a metrics collector


## Prerequisites

Before proceeding, review the following system requirements:
- A 64-bit Windows, Linux, or macOS environment
- [Podman](https://podman.io/getting-started/installation) and [podman-compose](https://github.com/containers/podman-compose) installed
- PowerShell 7+ (for running scripts)


## Part 1: Run Services

The SPIRE Server and Agent can be configured to emit telemetry by using a dedicated `telemetry { ... }` section in their configuration files. Currently, SPIRE supports Prometheus, StatsD, DogStatsD, M3 and In-Memory as metrics collectors. In this tutorial we'll show how to configure Prometheus and StatsD but simple configuration examples for the other collectors can be found in the [telemetry](https://github.com/spiffe/spire/blob/main/doc/telemetry_config.md) docs of the SPIRE project.

**Note:** The configuration changes needed to enable Prometheus and StatsD metrics collection from SPIRE are shown as snippets in this tutorial. However, all of these settings have already been configured. You don't have to edit any configuration files.

## Configure SPIRE to Emit Telemetry

The `telemetry` section supports the configuration of multiple collectors and for some collectors it is also possible to declare multiple instances.

The following snippet is from the [SPIRE Server configuration](spire/server/server.conf) file. The [SPIRE Agent configuration](spire/agent/agent.conf) file is configured the same way.

```console
telemetry {
   Prometheus {
      host = "spire-server"
      port = 8088
   }

   Statsd = [
      {
         address = "graphite-statsd:8125"
      },
   ]
}
```

### Prometheus Configuration in SPIRE

The first collector configured is Prometheus. Its configuration accepts two properties, the Prometheus server host which defaults to `localhost` and the Prometheus server port. These values are used by SPIRE to expose an endpoint which will be used by Prometheus to pull the metrics.

For the purpose of this tutorial we configured the host property using the hostname of the SPIRE Server (and Agent) but be aware that this configuration, which allows the SPIRE Server and Agent to listen for remote network connections, creates a security risk to SPIRE due to the open port. When applying a configuration like this in a production environment, the access to the endpoint should be tightly controlled. 

This configuration generates a warning message in the logs to alert the operator about this risk:

```console
level=warning msg="Agent is now configured to accept remote network connections for Prometheus stats collection. Please ensure access to this port is tightly controlled." subsystem_name=telemetry
```

### StatsD Configuration in SPIRE

The second collector configured is StatsD, which is one of the collectors that supports the configuration of multiple instances. For that reason, the configuration object expects a list of addresses. For this tutorial we define only one instance.
The address configured matches the StatsD instance running on the environment. We will see the details about this instance in a following section but for now it's worth noting that the address is formed by the hostname of the service and the default port for StatsD.

By configuring the address, SPIRE will be pushing metrics to the StatsD collector.

##  Graphite & StatsD Configuration

We use the official Docker image for Graphite and StatsD. This image already contains all the services necessary to collect and display metrics. For this tutorial we map the port `80` that belongs to the nginx proxy that reverse proxies the Graphite dashboard and the port `8125` where StatsD listens by default to the same external ports of 80 and 8125, respectively.
The `graphite-statsd` service definition is:

```console
  graphite-statsd:
    image: graphiteapp/graphite-statsd:1.1.7-6
    container_name: graphite
    hostname: graphite-statsd
    restart: always
    ports:
        - "80:80"
        - "8125:8125/udp"
```

The StatsD service will be available at `graphite-statsd:8125` as configured for SPIRE Server in the previous section.

## Prometheus Configuration

Due to the pull nature of Prometheus we need to configure the HTTP endpoint where it will scrape the metrics. We've already configured SPIRE to expose the HTTP endpoint via the telemetry configuration so now we need to indicate to Prometheus that it should collect metrics from that endpoint. We achieve this by setting the `target` option to the hostname of the SPIRE Server (or SPIRE Agent) and the correct port number (e.g. 8088 for the SPIRE Server and 8089 for the SPIRE Agent).

By default the HTTP resource path to fetch metrics from targets is `/metrics` but SPIRE does not expose metrics on that path. Instead, it does on the `/` path. These configurations are part of the [prometheus.yml](prometheus/prometheus.yml) configuration file.

```console
scrape_configs:
  - job_name: 'spire-server'
    metrics_path: '/'
    static_configs:
    - targets: ['spire-server:8088']

  - job_name: 'spire-agent'
    metrics_path: '/'
    static_configs:
    - targets: ['spire-agent:8089']
```

To run Prometheus we use the official Docker image and we mount the local directory `prometheus` to make the [prometheus.yml](prometheus/prometheus.yml) configuration file available at the container.

```console
prometheus:
  image: prom/prometheus:v2.20.1
  container_name: prometheus
  hostname: prometheus
  restart: always
  volumes:
    - ./prometheus:/etc/prometheus
  ports:
    - "9090:9090"
```


## Run the Scenario

Use the `set-env.ps1` script to run all the services that make up the scenario. The script starts the SPIRE Server, SPIRE Agent, Graphite-StatsD and Prometheus services.

Ensure that the current working directory is `scenarios/05-metrics/` and run:

```powershell
.\scripts\set-env.ps1
```

Once the script is completed, in another terminal run the following command to review the logs from all the services:
```powershell
podman-compose logs -f -t
```


## Part 2: Test the Deployments

Let's see some real data. Open your browser and navigate to `http://localhost/` to see the Graphite web UI and, on a different tab, navigate to `http://localhost:9090/` to access the Prometheus web UI.

To generate some data, let's create a workload registration entry:

```powershell
podman exec -it spiffe-spire-demo_spire-server_1 /opt/spire/bin/spire-server entry create `
    -spiffeID spiffe://example.org/myworkload `
    -parentID spiffe://example.org/myagent `
    -selector unix:uid:1000
```

And with this command we perform a request to fetch an SVID for that new workload. This request will serve to generate some metrics:

```powershell
podman exec -it spiffe-spire-demo_spire-agent_1 /opt/spire/bin/spire-agent api fetch x509 `
    -socketPath /opt/spire/sockets/workload_api.sock
```

Wait a couple of minutes while metrics are collected and then you can create graphs to review them.

There are different metrics exported by SPIRE that can be analyzed. As an example, the following images show a graph of the remaining TTL of each SVID fetched.

The graph using Graphite

![Graphite Graph][GraphiteGraph]

[GraphiteGraph]: images/graphite_graph.png "Graphite graph"


The same metric but this time shown using Prometheus UI

![Prometheus Graph][PrometheusGraph]

[PrometheusGraph]: images/prometheus_graph.png "Prometheus Graph"


## Cleanup

When you are finished running this tutorial, you can use the following PowerShell script to stop all the containers:

```powershell
.\scripts\clean-env.ps1
```

---

**Next:** Return to the [Scenario Index](../../README.md) to explore other scenarios.