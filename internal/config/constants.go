const (
	// Service information
	ServiceName    = "mirador-core"
	ServiceVersion = "v2.1.3"
	APIVersion     = "v1"

	// Default timeouts (milliseconds)
	DefaultHTTPTimeout      = 30000
	DefaultGRPCTimeout      = 30000
	DefaultCacheTimeout     = 5000
	DefaultShutdownTimeout  = 30000

	// Rate limiting defaults
	DefaultRateLimit       = 1000 // requests per minute per tenant
	DefaultBurstLimit      = 100  // burst allowance
	DefaultRateLimitWindow = 60   // seconds

	// Session management
	DefaultSessionTTL     = 86400 // 24 hours in seconds
	DefaultSessionCleanup = 3600  // 1 hour cleanup interval

	// Query limits
	DefaultQueryLimit       = 10000 // max results per query
	DefaultQueryTimeout     = 30    // seconds
	DefaultRangeQueryLimit  = 1000  // max series per range query
	DefaultLogQueryLimit    = 1000  // max logs per query

	// WebSocket limits
	DefaultWSMaxConnections = 1000
	DefaultWSMessageSize    = 1048576 // 1MB
	DefaultWSPingInterval   = 30      // seconds

	// Cache settings
	DefaultCacheTTL           = 300   // 5 minutes
	DefaultQueryCacheTTL      = 120   // 2 minutes for query results
	DefaultSessionCacheTTL    = 86400 // 24 hours for sessions
	DefaultMetadataCacheTTL   = 3600  // 1 hour for metadata

	// Health check intervals
	DefaultHealthCheckInterval = 30 // seconds
	DefaultHealthCheckTimeout  = 5  // seconds

	// AI Engine settings
	DefaultPredictionInterval    = 300   // 5 minutes
	DefaultCorrelationThreshold  = 0.85  // 85% confidence
	DefaultAnomalyThreshold      = 0.8   // 80% anomaly score
	DefaultFractureConfidence    = 0.7   // 70% minimum confidence

	// File size limits
	MaxConfigFileSize = 10485760 // 10MB
	MaxLogFileSize    = 104857600 // 100MB

	// Retry configurations
	DefaultRetryAttempts = 3
	DefaultRetryDelay    = 1000 // milliseconds
	DefaultBackoffFactor = 2.0
)

// Environment-specific constants
var (
	ProductionLogLevel = "warn"
	StagingLogLevel    = "info"
	DevelopmentLogLevel = "debug"
	TestLogLevel       = "error"

	ProductionCacheTTL = 600  // 10 minutes
	StagingCacheTTL    = 300  // 5 minutes
	DevelopmentCacheTTL = 60  // 1 minute
	TestCacheTTL       = 10   // 10 seconds
)
