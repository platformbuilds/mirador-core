import argparse
import random
import time
from opentelemetry import trace
from opentelemetry.trace import SpanKind
from opentelemetry.trace.status import Status, StatusCode
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.resources import Resource

provider = TracerProvider(resource=Resource.create({"service.name": "repro"}))
provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint="localhost:4317", insecure=True)))
trace.set_tracer_provider(provider)
tracer = trace.get_tracer(__name__)

LATENCY_PROFILES_MS = {
    "redis.write": {"normal": (5, 20), "anomaly": (80, 250), "anomaly_chance": 0.06},
    "kafka.write": {"normal": (10, 45), "anomaly": (120, 700), "anomaly_chance": 0.08},
    "cassandra.write": {"normal": (20, 90), "anomaly": (200, 1500), "anomaly_chance": 0.12},
    "redis.read": {"normal": (3, 15), "anomaly": (70, 220), "anomaly_chance": 0.05},
    "cassandra.read": {"normal": (15, 80), "anomaly": (180, 1300), "anomaly_chance": 0.10},
}

REQUEST_ANOMALY_CHANCE = 0.10
DEFAULT_REQUEST_COUNT = 50
DEFAULT_ERROR_RATE = 0.0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate synthetic OpenTelemetry traces with random latency, anomalies, and errors.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Examples:\n"
            "  python3 repro.py\n"
            "  python3 repro.py --anomaly-rate 0.30 --error-rate 0.10 --request-count 200\n"
            "  python3 repro.py --anomaly_rate 0.05 --error_rate 0.02 --request_count 1000\n"
        ),
    )
    parser.add_argument(
        "--anomaly-rate",
        "--anomaly_rate",
        dest="anomaly_rate",
        type=float,
        default=REQUEST_ANOMALY_CHANCE,
        help="Probability [0.0, 1.0] that a request span is anomalous (default: 0.10)",
    )
    parser.add_argument(
        "--request-count",
        "--request_count",
        dest="request_count",
        type=int,
        default=DEFAULT_REQUEST_COUNT,
        help="Number of request traces to generate (default: 50)",
    )
    parser.add_argument(
        "--error-rate",
        "--error_rate",
        dest="error_rate",
        type=float,
        default=DEFAULT_ERROR_RATE,
        help="Probability [0.0, 1.0] that a span is marked as error (default: 0.0)",
    )
    args = parser.parse_args()
    if not 0.0 <= args.anomaly_rate <= 1.0:
        parser.error("--anomaly-rate/--anomaly_rate must be between 0.0 and 1.0")
    if args.request_count <= 0:
        parser.error("--request-count/--request_count must be a positive integer")
    if not 0.0 <= args.error_rate <= 1.0:
        parser.error("--error-rate/--error_rate must be between 0.0 and 1.0")
    return args


def sample_duration_ms(span_name: str, force_slow: bool = False) -> tuple[float, bool]:
    profile = LATENCY_PROFILES_MS[span_name]
    is_anomaly = force_slow or (random.random() < profile["anomaly_chance"])
    low, high = profile["anomaly"] if is_anomaly else profile["normal"]
    return random.uniform(low, high), is_anomaly


def emit_dependency_span(span_name: str, message_id: str, force_slow: bool, error_chance: float) -> tuple[float, bool, bool]:
    duration_ms, is_anomaly = sample_duration_ms(span_name, force_slow=force_slow)
    is_error = random.random() < error_chance
    system, operation = span_name.split(".", 1)
    with tracer.start_as_current_span(span_name, kind=SpanKind.CLIENT) as span:
        span.set_attribute("messaging.message.id", message_id)
        span.set_attribute("dependency.system", system)
        span.set_attribute("dependency.operation", operation)
        span.set_attribute("anomaly_type", "slow" if is_anomaly else "normal")
        span.set_attribute("error", is_error)
        span.set_attribute("simulated.duration_ms", round(duration_ms, 2))
        if is_error:
            span.set_status(Status(StatusCode.ERROR, description="simulated dependency failure"))
            span.set_attribute("error.type", "synthetic_dependency_error")
        time.sleep(duration_ms / 1000)
    return duration_ms, is_anomaly, is_error


args = parse_args()
error_chance = args.error_rate


for i in range(args.request_count):
    message_id = f"msg-{i:04d}"
    # Fully random anomaly placement across runs.
    request_is_anomaly = random.random() < args.anomaly_rate
    request_is_error = random.random() < error_chance
    request_duration_ms = random.uniform(800, 2500) if request_is_anomaly else max(25, random.gauss(120, 35))
    with tracer.start_as_current_span("http.request", kind=SpanKind.SERVER) as span:
        span.set_attribute("messaging.message.id", message_id)
        span.set_attribute("anomaly_type", "slow" if request_is_anomaly else "normal")
        span.set_attribute("error", request_is_error)
        span.set_attribute("simulated.duration_ms", round(request_duration_ms, 2))
        if request_is_error:
            span.set_status(Status(StatusCode.ERROR, description="simulated request failure"))
            span.set_attribute("error.type", "synthetic_request_error")
        time.sleep(request_duration_ms / 1000)

        emitted = []
        for span_name in (
            "redis.write",
            "kafka.write",
            "cassandra.write",
            "redis.read",
            "cassandra.read",
        ):
            dep_duration_ms, dep_is_anomaly, dep_is_error = emit_dependency_span(
                span_name,
                message_id=message_id,
                force_slow=request_is_anomaly and random.random() < 0.5,
                error_chance=error_chance,
            )
            emitted.append((span_name, dep_duration_ms, dep_is_anomaly, dep_is_error))

    base_tag = "SLOW" if request_is_anomaly else "norm"
    err_tag = "ERR" if request_is_error else "ok "
    print(f"{base_tag:4s} {err_tag:4s} http.request      {request_duration_ms:8.0f}ms  id={message_id}")
    for name, latency, is_anomaly, is_error in emitted:
        level = "SLOW" if is_anomaly else "norm"
        dep_err_tag = "ERR" if is_error else "ok "
        print(f"{level:4s} {dep_err_tag:4s} {name:16s} {latency:8.0f}ms  id={message_id}")
    print("-" * 72)

provider.force_flush()
