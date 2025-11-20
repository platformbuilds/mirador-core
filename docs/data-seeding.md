# Data Seeding

This document describes the data seeding functionality for MIRADOR-CORE, which populates Weaviate with sample KPIs for demonstration and development purposes.

## Overview

The data seeding system provides:

1. **Sample KPIs**: Example KPI definitions demonstrating different types of metrics (technical and business)
2. **Default User Preferences**: Sensible defaults for user interface settings

## Usage

### Command Line Tool

Use the `schemactl` tool to seed data:

```bash
# Build the tool
go build -o bin/schemactl cmd/schemactl/main.go

# Seed default data
./bin/schemactl -mode=seed
```

### Environment Variables

Configure Weaviate connection:

```bash
export WEAVIATE_HOST=localhost
export WEAVIATE_PORT=8080
export WEAVIATE_SCHEME=http
```

### Makefile Integration

For local development, use the provided Makefile targets:

```bash
# Seed data in local development environment
make localdev-seed-data

# Full E2E with data seeding
make localdev
```

## Seeded Data

### Sample KPIs

#### Technical KPIs

1. **HTTP Request Duration**
   - Measures average HTTP request response times
   - Includes warning/critical thresholds
   - Sentiment: Negative (lower is better)

2. **Error Rate**
   - Tracks percentage of failed HTTP requests
   - Includes warning/critical thresholds
   - Sentiment: Negative (lower is better)

3. **System Uptime**
   - Monitors system availability percentage
   - Includes SLA-based thresholds
   - Sentiment: Positive (higher is better)

#### Business KPIs

1. **User Satisfaction Score**
   - Average user satisfaction from feedback
   - Includes target thresholds
   - Sentiment: Positive (higher is better)

2. **Revenue Per User**
   - Average monthly revenue per active user
   - Includes target thresholds
   - Sentiment: Positive (higher is better)

## Data Structure

All seeded data follows the established Weaviate schema:

- **KPIDefinition**: Stored in `KPIDefinition` class with query definitions and thresholds

## Idempotent Operations

The seeding operations are designed to be idempotent:

- Existing KPIs are not overwritten
- The system checks for existing data before creating new entries
- Multiple runs of the seeding command are safe

## Integration

The seeded data integrates with:

- **KPI API**: Sample KPIs are available for configuration
- **Query Engine**: KPIs can be used in unified queries and correlations

## Development

### Adding New Sample Data

To add new sample KPIs:

1. Update the `seedSampleKPIs()` function in `cmd/schemactl/main.go`
2. Follow the existing data structure patterns
3. Include appropriate thresholds and metadata
4. Test with `make localdev-seed-data`

### Modifying Existing Data

The seeding system checks for existing data and skips creation if items already exist. To modify seeded data:

1. Delete existing items via API or Weaviate console
2. Re-run the seeding command
3. Or manually update the seeding code and re-run

## Troubleshooting

### Weaviate Connection Issues

- Ensure Weaviate is running and accessible
- Check WEAVIATE_HOST and WEAVIATE_PORT environment variables

### Schema Mismatches

- Ensure Weaviate schema is up to date
- Run `go run cmd/server/main.go` first to initialize schema
- Check Weaviate logs for schema validation errors