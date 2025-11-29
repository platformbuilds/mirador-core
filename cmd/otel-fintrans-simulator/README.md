# OpenTelemetry Financial Transaction Simulator

## Purpose

This simulator generates realistic OpenTelemetry metrics, logs, and traces for a financial transaction processing system. It is used for:

- **Local development**: Testing Mirador Core's correlation and RCA engines with realistic telemetry data
- **Demo scenarios**: Showcasing platform capabilities with domain-specific observability patterns
- **Load testing**: Generating controlled telemetry volumes for performance testing

## Hardcoded Values - Exemption Rationale

**NOTE (HCB-008)**: This simulator contains hardcoded metric names, service names, and transaction attributes **by design**. This is an **approved exemption** from the AGENTS.md §3.6 rule against hardcoding for the following reasons:

### Why Hardcoding is Acceptable Here

1. **Simulation Fidelity**: The simulator must generate specific, realistic metric names (`transactions_total`, `transactions_failed_total`, etc.) that mirror actual financial services observability patterns.

2. **Not Production Code**: This is a development/testing tool, not part of the Mirador Core engine logic. It exists solely to generate synthetic data.

3. **Domain-Specific Vocabulary**: Financial transaction metrics (`transaction_amount_paisa_sum`, `db_latency_seconds`) represent a coherent domain model that would be nonsensical if abstracted.

4. **Self-Contained**: The simulator does not couple to or influence the behavior of the correlation/RCA engines, which remain registry-driven.

### Hardcoded Elements (Approved)

The following hardcoded elements are **permitted** in this simulator:

#### Metric Names
- `transactions_total`
- `transactions_failed_total`
- `db_ops_total`
- `kafka_produce_total`
- `kafka_consume_total`
- `transaction_latency_seconds`
- `db_latency_seconds`
- `transaction_amount_paisa_sum`
- `transaction_amount_paisa_count`

#### Service Names
- `api-gateway`
- `tps` (Transaction Processing Service)
- `postgres` (Database)
- `kafka` (Message queue)

#### Transaction Attributes
- `transaction.id`
- `transaction.type` (UPI, CREDIT_CARD, DEBIT_CARD, NETBANKING, WALLET)
- `transaction.status` (SUCCESS, FAILED, PENDING)
- `transaction.amount`
- `transaction.currency`

#### Resource Attributes
- `service.name`
- `service.version`
- `deployment.environment`
- `telemetry.sdk.name`
- `telemetry.sdk.language`
- `telemetry.sdk.version`

### Migration Path

If you need to **customize** the simulator's domain model:

1. Extract hardcoded values to a `simulator-config.yaml` file
2. Load configuration at startup
3. Generate telemetry based on loaded config

This is **not required** for the current use case but provides a path forward if simulation needs diversify.

### Relationship to Mirador Core

**Important**: The correlation and RCA engines in `internal/services/correlation_engine.go` and `internal/rca/` **must never** hardcode these metric/service names. Engines discover KPIs via:

- Stage-00 KPI registry (`internal/repo/kpi_repo.go`)
- EngineConfig (`internal/config/config.go`)
- Dynamic service graph discovery

The simulator merely **produces** data that matches patterns the engines **discover** at runtime.

## Usage

### Running Locally

```bash
# Build the simulator
go build -o bin/otel-fintrans-simulator cmd/otel-fintrans-simulator/main.go

# Run with default settings (sends to localhost:4317)
./bin/otel-fintrans-simulator

# Run with custom OTLP endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=http://my-collector:4317 ./bin/otel-fintrans-simulator

# Run with specific transaction rate
TRANSACTION_RATE=100 ./bin/otel-fintrans-simulator
```

### Configuration

Environment variables:

- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP collector endpoint (default: `http://localhost:4317`)
- `OTEL_SERVICE_NAME`: Service name for root traces (default: `api-gateway`)
- `TRANSACTION_RATE`: Transactions per second (default: `10`)
- `ERROR_RATE`: Percentage of failed transactions (default: `5`)
- `SIMULATION_DURATION`: How long to run (default: unlimited)

Configuration file (YAML)
-------------------------
The simulator now supports an optional YAML configuration file that controls telemetry names and failure scheduling.

By default the example config shipped with the tool is `simulator-config.yaml` (in this folder). Use `--config` to point to a custom config file:

```bash
# Use a custom YAML config
./bin/otel-fintrans-simulator --config ./cmd/otel-fintrans-simulator/simulator-config.yaml
```

The `failure` section supports a `bursty` mode and a list of `bursts` where the failure rate is multiplied for a time window. This enables more realistic, correlated failures.

Configuration example
---------------------
The bundled `simulator-config.yaml` (in this folder) contains a compact example which demonstrates:

- Overriding `service_names` used in spans/attributes
- Custom `metric_names` for all instrumented metrics
- A `failure` section that sets a base `rate`, chooses a mode (`bursty` recommended) and one or more `bursts` with `start`, `duration`, and `multiplier` values

Behavior notes
--------------
- If `--config` is not provided or the `failure` section is absent, the simulator falls back to the CLI flags `--failure-rate` and `--failure-mode` (original behaviour).
- If the YAML `failure.seed` is set, the simulator seeds randomness for deterministic runs, which is useful for reproducible demos/tests.

### Failure Scenarios

The simulator can inject realistic failure patterns:

```bash
# Inject database latency spike
INJECT_DB_LATENCY=true ./bin/otel-fintrans-simulator

# Inject Kafka producer failures
INJECT_KAFKA_FAILURES=true ./bin/otel-fintrans-simulator

# Inject cascading failures
INJECT_CASCADING_FAILURES=true ./bin/otel-fintrans-simulator
```

## Integration with Mirador Core

### Testing Correlation Engine

```bash
# 1. Start the simulator
./bin/otel-fintrans-simulator &

# 2. Wait for telemetry to accumulate (30s)
sleep 30

# 3. Query correlation API with time window
curl -X POST http://localhost:8010/api/v1/unified/correlate \
  -H "Content-Type: application/json" \
  -d '{
    "startTime": "2025-11-25T10:00:00Z",
    "endTime": "2025-11-25T10:15:00Z"
  }'
```

The correlation engine will:
1. Discover `transactions_failed_total` via KPI registry (not hardcoded!)
2. Identify correlated service latency patterns
3. Build service graph from observed trace telemetry
4. Return correlation result with confidence scores

### Testing RCA Engine

```bash
# Inject a specific failure
INJECT_DB_LATENCY=true ./bin/otel-fintrans-simulator &

# Query RCA for root cause analysis
curl -X POST http://localhost:8010/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d '{
    "startTime": "2025-11-25T10:05:00Z",
    "endTime": "2025-11-25T10:10:00Z"
  }'
```

## Contributing

When modifying the simulator:

1. **Keep domain fidelity**: Financial transaction vocabulary should remain realistic
2. **Don't pollute engines**: Never copy simulator hardcoded names into `internal/services/correlation_engine.go`
3. **Update this README**: Document any new hardcoded attributes/metrics
4. **Test with real Mirador**: Ensure correlation/RCA engines still work via registry discovery

## License

Apache 2.0 (see top-level LICENSE file)

## TODO / Next work items

Below is a proposed, ordered list of follow-up improvements and small issues we plan to take up one-by-one. These are intentionally scoped so we can address them in small, reviewable PRs that improve developer experience and simulation fidelity.

1. Make logging configurable and developer-friendly
  - Add a `--log` or `--log-output` flag that allows `stdout`/`nop` (default) and `otlp` so developers can see console logs locally by default.
  - Add an environment variable override (e.g., `SIM_LOG_OUTPUT`).

2. Graceful shutdown and context cancellation
  - Add a signal handler (SIGINT/SIGTERM) and a cancellable context to allow the simulator to stop quickly and shut down OTel providers cleanly.

3. Improve time-series scheduling and backfill behavior
  - Tweak scheduling so that future/edge offsets don't block for long periods (consider a bounded scheduler and non-blocking generation for unreachable timestamps).

4. Make telemetry configuration pluggable
  - Extract hardcoded metric names and service names into a `simulator-config.yaml` with a default for fidelity, and allow overriding at startup.

5. Fix metric instrument types & semantics
  - Replace `transaction_amount_paisa_count` (currently an UpDownCounter) with a monotonic `Counter` if only increments are intended.

6. Add reproducibility and seeding
  - Add a `--rand-seed` flag so tests and demo runs can generate reproducible streams.

7. Improve failure-mode realism
  - Add bursty/correlated failure patterns (e.g., sudden spike of Kafka failures) + a `failure-schedule` option to control when failures occur.

8. Add tests & CI automation for simulator build and basic runtime
  - Small unit tests for metrics initialization and a quick smoke `go test` that validates `initOTel` can be invoked in a test shim (using a fake collector or a no-op option).

9. Add a convenient local-dev Make target
  - `make localdev-sim` or `make run-simulator` that builds and runs the simulator against a local collector or logs to stdout for quick demos.

10. Document example `otel-collector` pipeline and how to point the simulator at it
   - Add clear examples showing collector config for receivers/exporters so new developers can get telemetry into Observability tooling quickly.

We'll take these up one at a time — tell me which item you'd like to tackle first and I’ll start a focused PR for it.
