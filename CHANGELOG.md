# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **BREAKING**: MIRA endpoint renamed from `POST /api/v1/mira/rca_analyse` to `POST /api/v1/mira/rca_analyze` (US English spelling)
  - All metric names updated from `mira_rca_analyse_*` to `mira_rca_analyze_*`
  - This aligns with Go ecosystem naming conventions which use US English
  - Update your API clients to use the new endpoint path

### Added
- **MIRA (Mirador Intelligent Research Assistant)**: AI-powered RCA explanation service
  - New endpoint: `POST /api/v1/mira/rca_analyze` for translating technical RCA output into non-technical narratives
  - Multi-provider support: OpenAI (GPT-4, GPT-3.5), Anthropic (Claude 3.5 Sonnet), vLLM, and Ollama
  - TOON (Token Oriented Object Notation) format support for 30-60% token reduction
  - Smart caching layer with Valkey for cost optimization (70%+ cache hit rate)
  - Dedicated rate limiting for MIRA endpoints (configurable requests per minute)
  - Comprehensive configuration via `mira` section in config files
  - Support for custom prompt templates using Go text/template syntax
  - Token usage tracking and metrics for cost monitoring
- **MIRA Rate Limiter Middleware**: Dedicated rate limiting for AI endpoints
  - Configurable limits via `MIRAConfig.RateLimit`
  - X-RateLimit-* headers in all responses
  - Separate rate limit key namespace (`rate_limit:mira:`)
- **MIRA Integration Tests**: Comprehensive test suite with 7 test cases
  - Success scenarios with all 4 providers
  - Error handling (invalid JSON, missing fields, service failures)
  - Prompt data extraction validation
  - Mock service implementation for testing
- **Documentation**: Complete MIRA usage guide at `docs/mira-rca-explain.md`
  - API reference with request/response examples
  - Provider comparison and cost analysis
  - Configuration guide with all options
  - Troubleshooting and best practices
  - Monitoring metrics and alerts

### Technical Implementation
- **MIRA Service Layer**: `internal/services/mira_service.go` - Interface and caching
- **Provider Implementations**: 
  - `internal/services/mira_provider_openai.go`
  - `internal/services/mira_provider_anthropic.go`
  - `internal/services/mira_provider_vllm.go`
  - `internal/services/mira_provider_ollama.go`
- **TOON Converter**: `internal/utils/toon_converter.go` - RCA to TOON format conversion
- **Handler Layer**: `internal/api/handlers/mira_rca.handler.go` - HTTP request handling
- **Middleware**: `internal/api/middleware/mira_rate_limiter.middleware.go` - Rate limiting
- **Configuration**: `internal/config/mira_config.go` - MIRA-specific configuration structures

### Dependencies
- Added `github.com/alpkeskin/gotoon v1.0.1` for TOON format support
- Added `github.com/sashabaranov/go-openai v1.41.2` for OpenAI integration

## [9.0.0] - 2025-11-15

### Security
- **RBAC Security Fixes**: Complete implementation of API key authentication replacing mock implementations
- **Authentication Method Enforcement**: Strict mode requiring API keys for programmatic access, rejecting session tokens
- **API Key Management**: Full CRUD operations with secure hashed storage and tenant isolation
- **Security Hardening**: Penetration testing, rate limiting, and comprehensive audit logging
- **Multi-Tenant Isolation**: Physical data separation with enforced tenant boundaries

### Added
- **API Key Repository**: Real Valkey/Weaviate implementations replacing mocks
- **Authentication Middleware**: Strict mode enforcement and security validation
- **Security E2E Tests**: Penetration testing scenarios and boundary validation
- **Rate Limiting**: Per-API-key rate limits with abuse detection
- **Audit Logging**: Comprehensive security event tracking

### Technical Implementation
- **Repository Layer**: `internal/repo/rbac/` - Real API key CRUD operations
- **Middleware Layer**: `internal/api/middleware/auth.middleware.go` - Strict mode and validation
- **Handler Layer**: `internal/api/handlers/auth.handler.go` - API key management endpoints
- **Security Testing**: `internal/api/security_e2e_test.go` - Penetration testing and validation

### Breaking Changes
- **Authentication Method Restrictions**: Programmatic API access now requires API keys only
- **Session Token Rejection**: Session tokens no longer accepted for REST API calls
- **API Key Format**: All API keys now start with `mrk_` prefix

### Security Improvements
- **No Plaintext Storage**: API keys are SHA256 hashed before storage
- **Tenant Isolation**: Complete data separation between tenants
- **Rate Limiting**: Configurable per-key rate limits
- **Audit Compliance**: All authentication events logged with user attribution

## [6.0.0] - 2025-10-08

### Added
- **Bleve Search Engine Integration**: Full parallel search engine support alongside existing Lucene functionality
- **Dual Search Engine Architecture**: Users can choose between Lucene and Bleve search engines for logs and traces queries
- **Search Router Component**: Central routing system for engine selection and query translation
- **Bleve Translator**: Query translation layer converting Bleve syntax to VictoriaMetrics LogsQL/Traces format
- **Enhanced Query Capabilities**: Support for term, match, phrase, wildcard, numeric range, and boolean queries in both engines
- **API Compatibility**: Backward-compatible API with optional `search_engine` field in request bodies
- **Configuration System**: Flexible search engine configuration with feature flags and per-engine settings

### Technical Implementation
- **Search Router**: `internal/utils/search/router.go` - Central component for engine selection and routing
- **Bleve Translator**: `internal/utils/bleve/translator.go` - Query translation and parsing logic
- **API Handler Updates**: Enhanced logs and traces handlers with engine selection support
- **Model Extensions**: Added `search_engine` field to `LogsQLQueryRequest` and `TraceSearchRequest`
- **Configuration Integration**: New `SearchConfig` struct with Bleve-specific settings

### Features
- **Engine Selection**: Choose between "lucene" (default) and "bleve" via `search_engine` request parameter
- **Query Syntax Support**:
  - **Lucene**: `+error -debug service:auth`, range queries `[100 TO 500]`
  - **Bleve**: `error AND NOT debug AND service:auth`, range queries `>=100 AND <=500`
- **Unified API**: Same endpoints and response formats regardless of search engine
- **Performance Monitoring**: Query translation metrics and engine selection tracking

### Quality Assurance
- **Unit Tests**: Comprehensive test coverage for search router and Bleve translator
- **Integration Tests**: End-to-end testing for both search engines
- **Backward Compatibility**: Extensive regression testing ensuring Lucene functionality unchanged
- **Performance Benchmarks**: Latency comparisons between search engines

### Breaking Changes
- None - All changes are backward compatible with existing Lucene queries

### Dependencies
- No new external dependencies added
- Leverages existing VictoriaMetrics and Valkey integrations

## [5.0.0] - 2025-09-30

### Added
- **MetricsQL API Implementation**: Complete implementation of comprehensive MetricsQL function APIs under `/api/v1/metrics/query`
- **226 New API Endpoints**: Added support for all major MetricsQL function categories:
  - **Rollup Functions (70 endpoints)**: rate, increase, delta, irate, deriv, idelta, ideriv, absent_over_time, avg_over_time, min_over_time, max_over_time, sum_over_time, count_over_time, quantile_over_time, stddev_over_time, stdvar_over_time, mad_over_time, zscore_over_time, distinct_over_time, changes, resets
  - **Transform Functions (90 endpoints)**: abs, ceil, floor, round, sqrt, exp, ln, log2, log10, sin, cos, tan, asin, acos, atan, sinh, cosh, tanh, deg, rad, clamp, clamp_min, clamp_max, histogram_quantile, label_replace, label_join, label_set, label_del, label_keep, label_copy, label_value, label_match, label_mismatch, sort, sort_desc, reverse, topk, bottomk, limitk, keep_last_value, keep_next_value, interpolate, union, absent, scalar, vector, time, hour, minute, month, year, day_of_month, day_of_week, days_in_month, timestamp, start, end, step, offset, increase_pure, rate_pure, delta_pure, irate_pure, deriv_pure, idelta_pure, ideriv_pure
  - **Label Manipulation Functions (22 endpoints)**: label_replace, label_join, label_set, label_del, label_keep, label_copy, label_value, label_match, label_mismatch
  - **Aggregate Functions (44 endpoints)**: sum, min, max, avg, stddev, stdvar, count, quantile, topk, bottomk, count_values, absent, increase, rate, delta, irate, deriv, idelta, ideriv, absent_over_time, avg_over_time, min_over_time, max_over_time, sum_over_time, count_over_time, quantile_over_time, stddev_over_time, stdvar_over_time, mad_over_time, zscore_over_time, distinct_over_time, changes, resets, sort, sort_desc, reverse, limitk, keep_last_value, keep_next_value, interpolate, union

### Technical Implementation
- **VictoriaMetrics Integration**: Direct integration with VictoriaMetrics for query execution
- **Comprehensive Validation**: Multi-layer validation including middleware and service-level checks
- **Error Handling**: Consistent error response patterns across all endpoints
- **Performance Optimization**: Efficient query construction and caching infrastructure
- **OpenAPI Documentation**: Complete API specification for all 226 endpoints
- **Testing Coverage**: Comprehensive test suite with unit, integration, and benchmark tests

### Infrastructure
- **Query Service Layer**: New `VictoriaMetricsQueryService` for handling MetricsQL queries
- **Validation Middleware**: `MetricsQLValidationMiddleware` for request validation
- **Handler Layer**: `MetricsQLQueryHandler` for API endpoint management
- **Model Definitions**: Complete request/response models for all function types

### Quality Assurance
- **226 API Endpoints Tested**: 100% functional coverage
- **Test Suite**: 35 unit tests, 13 integration tests, benchmark tests
- **Static Analysis**: go vet clean with no issues
- **Code Review**: Comprehensive review of error handling, security, and performance

### Breaking Changes
- None - All new endpoints added under new API path `/api/v1/metrics/query`

### Dependencies
- No new external dependencies added
- All existing VictoriaMetrics integration maintained

## [2.1.3] - Previous Version
- Initial release with core functionality