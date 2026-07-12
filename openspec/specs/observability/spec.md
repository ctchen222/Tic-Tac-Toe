# Observability Specification

## Purpose

Define the current telemetry production, collection, routing, storage, and visualization contract.

## Requirements

### Requirement: OpenTelemetry initialization

The game server MUST initialize OpenTelemetry trace, metric, and log providers before creating application repositories and serving requests.

#### Scenario: Initialize telemetry successfully

- **WHEN** the process can create an OTLP gRPC connection and providers
- **THEN** the global providers use resource service name `tic-tac-toe` and version `v0.1.0`

#### Scenario: Telemetry initialization fails

- **WHEN** provider or exporter construction returns an error
- **THEN** the process exits instead of starting without the configured telemetry baseline

### Requirement: OTLP signal export

The game server SHALL export traces, metrics, and logs over the configured OTLP gRPC connection, with traces sampled at a ratio of 0.1.

#### Scenario: Export application telemetry

- **WHEN** instrumented server, Hub, Room, repository, or logger operations emit signals
- **THEN** the SDK batches or periodically exports those signals to the configured Collector endpoint

### Requirement: Collector signal routing

The OpenTelemetry Collector SHALL receive OTLP signals, batch them, and route traces to Jaeger, metrics to its Prometheus exporter, and logs to Loki.

#### Scenario: Route a trace

- **WHEN** the Collector receives trace data
- **THEN** it batches and exports the trace to Jaeger and the debug exporter

#### Scenario: Route metrics

- **WHEN** the Collector receives metric data
- **THEN** it exposes those metrics through the Prometheus exporter on port 8889

#### Scenario: Route logs

- **WHEN** the Collector receives log data
- **THEN** it batches and pushes the logs to Loki and the debug exporter

### Requirement: Metrics scraping

Prometheus SHALL scrape the Collector's Prometheus exporter every 15 seconds.

#### Scenario: Collect game metrics

- **WHEN** the Collector exposes application metrics at `otel-collector:8889`
- **THEN** Prometheus stores the scraped time series under the configured collector job

### Requirement: Operational visualization

Grafana SHALL be provisioned with Prometheus as its default metric source and Loki as its log source.

#### Scenario: Query metrics and logs

- **WHEN** an operator opens Grafana
- **THEN** dashboards and exploration can query Prometheus metrics and Loki logs through provisioned data sources

### Requirement: Telemetry shutdown

The game server SHALL flush trace, metric, and log providers and close the shared gRPC connection during graceful shutdown.

#### Scenario: Shut down the server

- **WHEN** the process receives an interrupt or termination signal
- **THEN** the HTTP server and telemetry providers are shut down within bounded contexts before exit
