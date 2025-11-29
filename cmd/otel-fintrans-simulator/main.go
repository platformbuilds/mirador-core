package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	currencyINR = "INR"
)

type FailureMode string

const (
	FailureModeNone       FailureMode = "none"
	FailureModeKafka      FailureMode = "kafka"
	FailureModeCassandra  FailureMode = "cassandra"
	FailureModeKeyDB      FailureMode = "keydb"
	FailureModeAPIGateway FailureMode = "api-gateway"
	FailureModeTPS        FailureMode = "tps"
	FailureModeMixed      FailureMode = "mixed"
)

type Simulator struct {
	tracer         trace.Tracer
	meter          metric.Meter
	logger         *zap.Logger
	transactions   int
	failureMode    FailureMode
	failureRate    float64
	minAmountPaise int64
	maxAmountPaise int64
	concurrency    int

	// Time series configuration
	timeWindowDuration time.Duration // Total duration to spread data over
	dataInterval       time.Duration // Time between data points
	startTimeOffset    time.Duration // How far back to start (e.g., -15m means 15 minutes ago)

	// Metric instruments
	transactionsTotal       metric.Int64Counter
	transactionsFailedTotal metric.Int64Counter
	dbOpsTotal              metric.Int64Counter
	kafkaProduceTotal       metric.Int64Counter
	kafkaConsumeTotal       metric.Int64Counter
	transactionLatency      metric.Float64Histogram
	dbLatency               metric.Float64Histogram
	transactionAmountSum    metric.Int64Counter
	transactionAmountCount  metric.Int64UpDownCounter
	// runtime configuration
	telemCfg     TelemetryConfig
	failureSched *failureScheduler
}

type TransactionResult struct {
	TransactionID string
	AmountPaise   int64
	CustomerID    string
	Channel       string
	FinalStatus   string
	ErrorReason   string
}

func main() {
	var (
		transactions       = flag.Int("transactions", 100, "Number of transactions to simulate")
		failureModeStr     = flag.String("failure-mode", "mixed", "Failure mode: none, kafka, cassandra, keydb, api-gateway, tps, mixed")
		failureRate        = flag.Float64("failure-rate", 0.1, "Failure rate (0.0-1.0)")
		minAmountPaise     = flag.Int64("min-amount-paisa", 10000, "Minimum transaction amount in paise (₹100.00)")
		maxAmountPaise     = flag.Int64("max-amount-paisa", 1000000, "Maximum transaction amount in paise (₹10,000.00)")
		otlpEndpoint       = flag.String("otlp-endpoint", "localhost:4317", "OTLP endpoint")
		otlpInsecure       = flag.Bool("otlp-insecure", true, "Use insecure OTLP connection")
		concurrency        = flag.Int("concurrency", 10, "Number of concurrent transactions")
		timeWindowStr      = flag.String("time-window", "0s", "Time window to spread data over (e.g., 15m, 1h, 0s=instant)")
		dataIntervalStr    = flag.String("data-interval", "30s", "Time interval between data points (e.g., 10s, 1m)")
		startTimeOffsetStr = flag.String("start-time-offset", "0s", "Start time offset from now (e.g., -15m for 15 minutes ago, 0s=now)")
		configPath         = flag.String("config", "", "Path to optional simulator YAML configuration file")
	)
	flag.Parse()

	failureMode := FailureMode(*failureModeStr)
	if failureMode != FailureModeNone && failureMode != FailureModeKafka &&
		failureMode != FailureModeCassandra && failureMode != FailureModeKeyDB &&
		failureMode != FailureModeAPIGateway && failureMode != FailureModeTPS &&
		failureMode != FailureModeMixed {
		log.Fatalf("Invalid failure mode: %s", failureMode)
	}

	if *failureRate < 0 || *failureRate > 1 {
		log.Fatalf("Failure rate must be between 0.0 and 1.0")
	}

	// Parse time durations
	timeWindowDuration, err := time.ParseDuration(*timeWindowStr)
	if err != nil {
		log.Fatalf("Invalid time-window: %v", err)
	}

	dataInterval, err := time.ParseDuration(*dataIntervalStr)
	if err != nil {
		log.Fatalf("Invalid data-interval: %v", err)
	}

	startTimeOffset, err := time.ParseDuration(*startTimeOffsetStr)
	if err != nil {
		log.Fatalf("Invalid start-time-offset: %v", err)
	}

	// Initialize OpenTelemetry
	ctx := context.Background()
	shutdown, err := initOTel(ctx, *otlpEndpoint, *otlpInsecure)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown OpenTelemetry: %v", err)
		}
	}()

	// Create simulator
	sim := &Simulator{
		tracer:             otel.Tracer("fintrans-simulator"),
		meter:              otel.Meter("fintrans-simulator"),
		logger:             zap.NewNop(), // Will be set after OTel init
		transactions:       *transactions,
		failureMode:        failureMode,
		failureRate:        *failureRate,
		minAmountPaise:     *minAmountPaise,
		maxAmountPaise:     *maxAmountPaise,
		concurrency:        *concurrency,
		timeWindowDuration: timeWindowDuration,
		dataInterval:       dataInterval,
		startTimeOffset:    startTimeOffset,
	}

	// Load optional config file
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg != nil {
		sim.telemCfg = cfg.Telemetry

		// If the config contains a failure section, and has a rate/bursts/mode, build scheduler
		if cfg.Failure.Rate > 0 || len(cfg.Failure.Bursts) > 0 || cfg.Failure.Mode != "" {
			fs, err := newFailureScheduler(cfg.Failure, time.Now().Add(sim.startTimeOffset))
			if err != nil {
				log.Fatalf("failed to create failure scheduler: %v", err)
			}
			sim.failureSched = fs

			// If a seed is provided, ensure package-level randomness is seeded for deterministic behavior of other RNG use
			if cfg.Failure.Seed != nil {
				rand.Seed(*cfg.Failure.Seed)
			}
		}
	}

	// Initialize metrics
	if err := sim.initMetrics(ctx); err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}

	// Initialize logger with OTel bridge
	sim.initLogger()

	log.Printf("Starting financial transaction simulator with %d transactions, concurrency: %d, failure mode: %s, rate: %.2f",
		*transactions, *concurrency, failureMode, *failureRate)

	if timeWindowDuration > 0 {
		log.Printf("Time series mode: window=%v, interval=%v, start_offset=%v",
			timeWindowDuration, dataInterval, startTimeOffset)
	} else {
		log.Printf("Instant mode: all transactions at current time")
	}

	// Run simulation
	sim.runSimulation(ctx)

	log.Println("Simulation completed")
}

func initOTel(ctx context.Context, endpoint string, insecureConn bool) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("fintrans-simulator"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	opts := []grpc.DialOption{}
	if insecureConn {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Trace exporter
	traceOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(opts...),
	}
	if insecureConn {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}
	traceExporter, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	// Metric exporter
	metricOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithDialOption(opts...),
	}
	if insecureConn {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// Log exporter
	logOpts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(endpoint),
		otlploggrpc.WithDialOption(opts...),
	}
	if insecureConn {
		logOpts = append(logOpts, otlploggrpc.WithInsecure())
	}
	logExporter, err := otlploggrpc.New(ctx, logOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create log exporter: %w", err)
	}

	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(logProvider)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func(ctx context.Context) error {
		var errs []error
		if err := tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := logProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return fmt.Errorf("shutdown errors: %v", errs)
		}
		return nil
	}, nil
}

func (s *Simulator) initMetrics(ctx context.Context) error {
	var err error

	txTotalName := "transactions_total"
	if s.telemCfg.MetricNames.TransactionsTotal != "" {
		txTotalName = s.telemCfg.MetricNames.TransactionsTotal
	}
	s.transactionsTotal, err = s.meter.Int64Counter(txTotalName,
		metric.WithDescription("Total number of transactions"))
	if err != nil {
		return err
	}

	txFailedName := "transactions_failed_total"
	if s.telemCfg.MetricNames.TransactionsFailed != "" {
		txFailedName = s.telemCfg.MetricNames.TransactionsFailed
	}
	s.transactionsFailedTotal, err = s.meter.Int64Counter(txFailedName,
		metric.WithDescription("Total number of failed transactions"))
	if err != nil {
		return err
	}

	dbOpsName := "db_ops_total"
	if s.telemCfg.MetricNames.DBOpsTotal != "" {
		dbOpsName = s.telemCfg.MetricNames.DBOpsTotal
	}
	s.dbOpsTotal, err = s.meter.Int64Counter(dbOpsName,
		metric.WithDescription("Total database operations"))
	if err != nil {
		return err
	}

	kafkaProduceName := "kafka_produce_total"
	if s.telemCfg.MetricNames.KafkaProduceTotal != "" {
		kafkaProduceName = s.telemCfg.MetricNames.KafkaProduceTotal
	}
	s.kafkaProduceTotal, err = s.meter.Int64Counter(kafkaProduceName,
		metric.WithDescription("Total Kafka produce operations"))
	if err != nil {
		return err
	}

	kafkaConsumeName := "kafka_consume_total"
	if s.telemCfg.MetricNames.KafkaConsumeTotal != "" {
		kafkaConsumeName = s.telemCfg.MetricNames.KafkaConsumeTotal
	}
	s.kafkaConsumeTotal, err = s.meter.Int64Counter(kafkaConsumeName,
		metric.WithDescription("Total Kafka consume operations"))
	if err != nil {
		return err
	}

	txLatencyName := "transaction_latency_seconds"
	if s.telemCfg.MetricNames.TransactionLatency != "" {
		txLatencyName = s.telemCfg.MetricNames.TransactionLatency
	}
	s.transactionLatency, err = s.meter.Float64Histogram(txLatencyName,
		metric.WithDescription("Transaction processing latency"))
	if err != nil {
		return err
	}

	dbLatencyName := "db_latency_seconds"
	if s.telemCfg.MetricNames.DBLatency != "" {
		dbLatencyName = s.telemCfg.MetricNames.DBLatency
	}
	s.dbLatency, err = s.meter.Float64Histogram(dbLatencyName,
		metric.WithDescription("Database operation latency"))
	if err != nil {
		return err
	}

	txAmountSumName := "transaction_amount_paisa_sum"
	if s.telemCfg.MetricNames.TransactionAmountSum != "" {
		txAmountSumName = s.telemCfg.MetricNames.TransactionAmountSum
	}
	s.transactionAmountSum, err = s.meter.Int64Counter(txAmountSumName,
		metric.WithDescription("Sum of transaction amounts in paise"))
	if err != nil {
		return err
	}

	txAmountCountName := "transaction_amount_paisa_count"
	if s.telemCfg.MetricNames.TransactionAmountCount != "" {
		txAmountCountName = s.telemCfg.MetricNames.TransactionAmountCount
	}
	s.transactionAmountCount, err = s.meter.Int64UpDownCounter(txAmountCountName,
		metric.WithDescription("Count of transactions for amount metrics"))
	if err != nil {
		return err
	}

	return nil
}

func (s *Simulator) initLogger() {
	// Create a logger that outputs to OTel
	// We'll use a no-op logger for now since OTel handles the logging
	s.logger = zap.NewNop()
}

func (s *Simulator) runSimulation(ctx context.Context) {
	// Calculate time distribution
	var timestamps []time.Time

	if s.timeWindowDuration > 0 {
		// Time series mode: distribute transactions over the time window
		startTime := time.Now().Add(s.startTimeOffset)

		// Calculate number of intervals
		numIntervals := int(s.timeWindowDuration / s.dataInterval)
		if numIntervals == 0 {
			numIntervals = 1
		}

		// Distribute transactions evenly across intervals
		txPerInterval := s.transactions / numIntervals
		remainder := s.transactions % numIntervals

		txIndex := 0
		for i := 0; i < numIntervals; i++ {
			intervalTime := startTime.Add(time.Duration(i) * s.dataInterval)
			txCount := txPerInterval
			if i < remainder {
				txCount++ // Distribute remainder across first intervals
			}

			// All transactions in this interval get the same timestamp
			for j := 0; j < txCount; j++ {
				timestamps = append(timestamps, intervalTime)
				txIndex++
			}
		}

		// Sort by timestamp to ensure chronological execution
		// Already sorted by construction, but explicit for clarity

		log.Printf("Generated %d transactions across %d intervals (%v each)",
			len(timestamps), numIntervals, s.dataInterval)

		// Execute transactions in chronological order, sleeping to match timestamps
		for i := 0; i < s.transactions; i++ {
			targetTime := timestamps[i]
			sleepDuration := time.Until(targetTime)

			if sleepDuration > 0 {
				log.Printf("Waiting %v until next interval...", sleepDuration)
				time.Sleep(sleepDuration)
			}

			// Execute all transactions for this timestamp
			intervalStart := i
			intervalEnd := i + 1
			for intervalEnd < len(timestamps) && timestamps[intervalEnd].Equal(targetTime) {
				intervalEnd++
			}

			// Execute this batch concurrently
			var wg sync.WaitGroup
			sem := make(chan struct{}, s.concurrency)

			for j := intervalStart; j < intervalEnd; j++ {
				wg.Add(1)
				go func(txID int) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					s.simulateTransaction(ctx, fmt.Sprintf("tx-%06d", txID))
				}(j)
			}

			wg.Wait()

			// Skip to next interval
			i = intervalEnd - 1
		}
	} else {
		// Instant mode: all transactions at current time
		var wg sync.WaitGroup
		sem := make(chan struct{}, s.concurrency)

		for i := 0; i < s.transactions; i++ {
			wg.Add(1)
			go func(txID int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				s.simulateTransaction(ctx, fmt.Sprintf("tx-%06d", txID))
			}(i)
		}

		wg.Wait()
	}
}

func (s *Simulator) simulateTransaction(ctx context.Context, transactionID string) {
	start := time.Now()

	// Generate transaction details
	amountPaise := s.minAmountPaise + rand.Int63n(s.maxAmountPaise-s.minAmountPaise+1)
	customerID := fmt.Sprintf("cust-%04d", rand.Intn(10000))
	channel := []string{"mobile", "web", "api"}[rand.Intn(3)]

	// Determine if this transaction should fail
	var shouldFail bool
	if s.failureSched != nil {
		shouldFail = s.failureSched.ShouldFail(time.Now())
	} else {
		shouldFail = rand.Float64() < s.failureRate
	}
	var failureComponent string
	if shouldFail {
		switch s.failureMode {
		case FailureModeKafka:
			failureComponent = "kafka"
		case FailureModeCassandra:
			failureComponent = "cassandra"
		case FailureModeKeyDB:
			failureComponent = "keydb"
		case FailureModeAPIGateway:
			failureComponent = "api-gateway"
		case FailureModeTPS:
			failureComponent = "tps"
		case FailureModeMixed:
			components := []string{"kafka", "cassandra", "keydb", "api-gateway", "tps"}
			failureComponent = components[rand.Intn(len(components))]
		}
	}

	// Start root span for API Gateway
	apiGatewayName := "api-gateway"
	if s.telemCfg.ServiceNames.APIGateway != "" {
		apiGatewayName = s.telemCfg.ServiceNames.APIGateway
	}

	ctx, rootSpan := s.tracer.Start(ctx, apiGatewayName+".receive_transaction",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("customer_id", customerID),
			attribute.String("channel", channel),
			attribute.Int64("amount_paisa", amountPaise),
			attribute.String("currency", currencyINR),
		))
	defer rootSpan.End()

	// Log API Gateway request received
	s.logger.Info("Transaction request received",
		zap.String("transaction_id", transactionID),
		zap.String("service_name", apiGatewayName),
		zap.Int64("amount_paisa", amountPaise),
		zap.String("currency", currencyINR),
		zap.String("severity", "INFO"),
	)

	// Simulate API Gateway auth check
	if failureComponent == "api-gateway" && shouldFail {
		rootSpan.SetStatus(codes.Error, "Auth validation failed")
		rootSpan.SetAttributes(
			attribute.String("final_status", "error"),
			attribute.String("error_type", "auth_failure"),
		)
		s.transactionsFailedTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("service_name", apiGatewayName)))
		s.logger.Error("Auth validation failed",
			zap.String("transaction_id", transactionID),
			zap.String("service_name", apiGatewayName),
			zap.Int64("amount_paisa", amountPaise),
			zap.String("currency", currencyINR),
			zap.String("severity", "ERROR"),
			zap.String("failure_reason", "auth_failure"),
		)
		return
	}

	// Call TPS
	tpsName := "tps"
	if s.telemCfg.ServiceNames.TPS != "" {
		tpsName = s.telemCfg.ServiceNames.TPS
	}

	ctx, tpsClientSpan := s.tracer.Start(ctx, apiGatewayName+".call_tps",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", apiGatewayName),
			attribute.String("peer.service", tpsName),
		))
	result := s.simulateTPS(ctx, transactionID, amountPaise, customerID, failureComponent, shouldFail)
	tpsClientSpan.End()

	// API Gateway consumes from Kafka and writes to Cassandra
	if result.FinalStatus == "success" {
		s.simulateAPIGatewayKafkaConsumer(ctx, transactionID, amountPaise, failureComponent == "kafka" && shouldFail)
	}

	// Update root span
	rootSpan.SetAttributes(
		attribute.String("final_status", result.FinalStatus),
	)
	if result.FinalStatus == "error" {
		rootSpan.SetStatus(codes.Error, result.ErrorReason)
	}

	// Record final metrics
	s.transactionsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("service_name", apiGatewayName)))
	s.transactionAmountSum.Add(ctx, result.AmountPaise, metric.WithAttributes(attribute.String("currency", currencyINR)))
	s.transactionAmountCount.Add(ctx, 1, metric.WithAttributes(attribute.String("currency", currencyINR)))

	if result.FinalStatus == "error" {
		s.transactionsFailedTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("service_name", apiGatewayName)))
	}

	duration := time.Since(start).Seconds()
	s.transactionLatency.Record(ctx, duration, metric.WithAttributes(attribute.String("service_name", apiGatewayName)))
}

func (s *Simulator) simulateTPS(ctx context.Context, transactionID string, amountPaise int64, customerID string, failureComponent string, shouldFail bool) TransactionResult {
	tpsName := "tps"
	if s.telemCfg.ServiceNames.TPS != "" {
		tpsName = s.telemCfg.ServiceNames.TPS
	}

	ctx, span := s.tracer.Start(ctx, tpsName+".process_transaction",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", tpsName),
		))
	defer span.End()

	result := TransactionResult{
		TransactionID: transactionID,
		AmountPaise:   amountPaise,
		CustomerID:    customerID,
		FinalStatus:   "success",
	}

	// Simulate 22 TPS steps
	for step := 1; step <= 22; step++ {
		stepCtx, stepSpan := s.tracer.Start(ctx, fmt.Sprintf(tpsName+".step_%02d", step),
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("transaction_id", transactionID),
				attribute.String("service_name", tpsName),
				attribute.Int("step_number", step),
			))

		// Simulate different operations based on step
		switch {
		case step <= 5:
			// Auth and validation steps - use Cassandra
			if err := s.simulateCassandraOp(stepCtx, transactionID, "auth_lookup", failureComponent == "cassandra" && shouldFail); err != nil {
				result.FinalStatus = "error"
				result.ErrorReason = "cassandra_auth_failure"
				stepSpan.SetStatus(codes.Error, err.Error())
				s.logger.Error("TPS step failed",
					zap.String("transaction_id", transactionID),
					zap.String("service_name", tpsName),
					zap.Int("step_number", step),
					zap.Int64("amount_paisa", amountPaise),
					zap.String("currency", currencyINR),
					zap.String("severity", "ERROR"),
					zap.String("failure_reason", "cassandra_auth_failure"),
				)
				stepSpan.End()
				return result
			}
		case step <= 15:
			// Processing steps - use KeyDB for state
			if err := s.simulateKeyDBOp(stepCtx, transactionID, fmt.Sprintf("state_update_%d", step), failureComponent == "keydb" && shouldFail); err != nil {
				result.FinalStatus = "error"
				result.ErrorReason = "keydb_state_failure"
				stepSpan.SetStatus(codes.Error, err.Error())
				s.logger.Error("TPS step failed",
					zap.String("transaction_id", transactionID),
					zap.String("service_name", tpsName),
					zap.Int("step_number", step),
					zap.Int64("amount_paisa", amountPaise),
					zap.String("currency", currencyINR),
					zap.String("severity", "ERROR"),
					zap.String("failure_reason", "keydb_state_failure"),
				)
				stepSpan.End()
				return result
			}
		case step == 16:
			// Final validation - use Cassandra
			if err := s.simulateCassandraOp(stepCtx, transactionID, "final_validation", failureComponent == "cassandra" && shouldFail); err != nil {
				result.FinalStatus = "error"
				result.ErrorReason = "cassandra_validation_failure"
				stepSpan.SetStatus(codes.Error, err.Error())
				stepSpan.End()
				return result
			}
		case step >= 17 && step <= 20:
			// Internal processing
			time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
		case step == 21:
			// Produce to Kafka
			if err := s.simulateKafkaProduce(stepCtx, transactionID, amountPaise, failureComponent == "kafka" && shouldFail); err != nil {
				result.FinalStatus = "error"
				result.ErrorReason = "kafka_produce_failure"
				stepSpan.SetStatus(codes.Error, err.Error())
				s.logger.Error("TPS step failed",
					zap.String("transaction_id", transactionID),
					zap.String("service_name", tpsName),
					zap.Int("step_number", step),
					zap.Int64("amount_paisa", amountPaise),
					zap.String("currency", currencyINR),
					zap.String("severity", "ERROR"),
					zap.String("failure_reason", "kafka_produce_failure"),
				)
				stepSpan.End()
				return result
			}
		case step == 22:
			// Final commit
			time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)
		}

		stepSpan.End()
	}

	return result
}

func (s *Simulator) simulateCassandraOp(ctx context.Context, transactionID, operation string, shouldFail bool) error {
	cassName := "cassandra-client"
	if s.telemCfg.ServiceNames.CassandraClient != "" {
		cassName = s.telemCfg.ServiceNames.CassandraClient
	}

	ctx, span := s.tracer.Start(ctx, "cassandra."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", cassName),
			attribute.String("db.system", "cassandra"),
			attribute.String("db.operation", operation),
		))
	defer span.End()

	start := time.Now()

	if shouldFail {
		span.SetStatus(codes.Error, "Cassandra operation failed")
		s.logger.Error("Cassandra operation failed",
			zap.String("transaction_id", transactionID),
			zap.String("service_name", cassName),
			zap.String("severity", "ERROR"),
			zap.String("failure_reason", "cassandra_timeout"),
		)
		return fmt.Errorf("cassandra operation failed")
	}

	// Simulate latency
	time.Sleep(time.Duration(20+rand.Intn(30)) * time.Millisecond)
	duration := time.Since(start).Seconds()

	s.dbOpsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("db_system", "cassandra")))
	s.dbLatency.Record(ctx, duration, metric.WithAttributes(attribute.String("db_system", "cassandra")))

	return nil
}

func (s *Simulator) simulateKeyDBOp(ctx context.Context, transactionID, operation string, shouldFail bool) error {
	keyName := "keydb-client"
	if s.telemCfg.ServiceNames.KeyDBClient != "" {
		keyName = s.telemCfg.ServiceNames.KeyDBClient
	}

	ctx, span := s.tracer.Start(ctx, "keydb."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", keyName),
			attribute.String("db.system", "valkey"),
			attribute.String("db.operation", operation),
		))
	defer span.End()

	start := time.Now()

	if shouldFail {
		span.SetStatus(codes.Error, "KeyDB operation failed")
		s.logger.Error("KeyDB operation failed",
			zap.String("transaction_id", transactionID),
			zap.String("service_name", keyName),
			zap.String("severity", "ERROR"),
			zap.String("failure_reason", "keydb_connection_error"),
		)
		return fmt.Errorf("keydb operation failed")
	}

	// Simulate latency
	time.Sleep(time.Duration(5+rand.Intn(15)) * time.Millisecond)
	duration := time.Since(start).Seconds()

	s.dbOpsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("db_system", "valkey")))
	s.dbLatency.Record(ctx, duration, metric.WithAttributes(attribute.String("db_system", "valkey")))

	return nil
}

func (s *Simulator) simulateKafkaProduce(ctx context.Context, transactionID string, amountPaise int64, shouldFail bool) error {
	kpName := "kafka-producer"
	if s.telemCfg.ServiceNames.KafkaProducer != "" {
		kpName = s.telemCfg.ServiceNames.KafkaProducer
	}

	ctx, span := s.tracer.Start(ctx, "kafka.produce",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", kpName),
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.operation", "produce"),
			attribute.String("messaging.destination", "transaction-log"),
		))
	defer span.End()

	start := time.Now()

	if shouldFail {
		span.SetStatus(codes.Error, "Kafka produce failed")
		return fmt.Errorf("kafka produce failed")
	}

	// Simulate latency
	time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
	duration := time.Since(start).Seconds()

	s.kafkaProduceTotal.Add(ctx, 1)
	s.dbLatency.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "kafka_produce")))

	return nil
}

func (s *Simulator) simulateKafkaConsumer(ctx context.Context, transactionID string, amountPaise int64, shouldFail bool) {
	kcName := "kafka-consumer"
	if s.telemCfg.ServiceNames.KafkaConsumer != "" {
		kcName = s.telemCfg.ServiceNames.KafkaConsumer
	}

	ctx, span := s.tracer.Start(ctx, "kafka.consume",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", kcName),
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.operation", "consume"),
			attribute.String("messaging.destination", "transaction-log"),
		))
	defer span.End()

	start := time.Now()

	if shouldFail {
		span.SetStatus(codes.Error, "Kafka consume failed")
		return
	}

	// Simulate processing delay
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

	// Write final state to Cassandra
	if err := s.simulateCassandraOp(ctx, transactionID, "final_state_write", false); err != nil {
		span.SetStatus(codes.Error, "Final state write failed")
		return
	}

	duration := time.Since(start).Seconds()
	s.kafkaConsumeTotal.Add(ctx, 1)
	s.dbLatency.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "kafka_consume")))
}

func (s *Simulator) simulateAPIGatewayKafkaConsumer(ctx context.Context, transactionID string, amountPaise int64, shouldFail bool) {
	// API Gateway consumes from Kafka
	agName := "api-gateway"
	if s.telemCfg.ServiceNames.APIGateway != "" {
		agName = s.telemCfg.ServiceNames.APIGateway
	}

	ctx, consumerSpan := s.tracer.Start(ctx, agName+".kafka_consume",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", agName),
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.operation", "consume"),
			attribute.String("messaging.destination", "transaction-log"),
		))
	defer consumerSpan.End()

	start := time.Now()

	if shouldFail {
		consumerSpan.SetStatus(codes.Error, "Kafka consume failed")
		return
	}

	// Simulate processing delay
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

	// API Gateway writes final state to Cassandra
	if err := s.simulateAPIGatewayCassandraWrite(ctx, transactionID, "final_state_write", false); err != nil {
		consumerSpan.SetStatus(codes.Error, "Final state write failed")
		return
	}

	duration := time.Since(start).Seconds()
	s.kafkaConsumeTotal.Add(ctx, 1)
	s.dbLatency.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "kafka_consume")))
}

func (s *Simulator) simulateAPIGatewayCassandraWrite(ctx context.Context, transactionID, operation string, shouldFail bool) error {
	agName := "api-gateway"
	if s.telemCfg.ServiceNames.APIGateway != "" {
		agName = s.telemCfg.ServiceNames.APIGateway
	}

	ctx, span := s.tracer.Start(ctx, agName+".cassandra_write",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("service_name", agName),
			attribute.String("db.system", "cassandra"),
			attribute.String("db.operation", operation),
		))
	defer span.End()

	start := time.Now()

	if shouldFail {
		span.SetStatus(codes.Error, "Cassandra write failed")
		return fmt.Errorf("cassandra write failed")
	}

	// Simulate latency
	time.Sleep(time.Duration(20+rand.Intn(30)) * time.Millisecond)
	duration := time.Since(start).Seconds()

	s.dbOpsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("db_system", "cassandra")))
	s.dbLatency.Record(ctx, duration, metric.WithAttributes(attribute.String("db_system", "cassandra")))

	return nil
}
