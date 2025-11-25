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
