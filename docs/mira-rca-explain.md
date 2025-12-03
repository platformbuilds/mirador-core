# MIRA RCA Explanation Service

## Overview

MIRA (Mirador Intelligent Research Assistant) is an AI-powered service that translates technical Root Cause Analysis (RCA) output into non-technical, narrative explanations. It makes incident investigations accessible to business stakeholders, executives, and non-technical team members.

## Purpose

Traditional RCA output contains:
- Complex correlation graphs
- Statistical metrics (correlation coefficients, p-values, suspicion scores)
- Technical KPI names and metric queries
- Multiple causal chains with temporal rings

While this data is valuable for SREs and engineers, it's often incomprehensible to business stakeholders. MIRA bridges this gap by:

1. **Translating technical terminology** into plain language
2. **Synthesizing multiple causal chains** into coherent narratives
3. **Highlighting business impact** and root causes
4. **Providing actionable insights** for non-technical audiences

## API Endpoint

```
POST /api/v1/mira/rca_analyze
```

### Request Format

The endpoint accepts the full RCA response from `/api/v1/unified/rca`:

```json
{
  "rcaData": {
    "status": "success",
    "data": {
      "impact": {
        "impactService": "payment-service",
        "metricName": "payment_failures_total",
        "severity": 0.85,
        "timeStartStr": "2025-12-03T07:30:00Z",
        "timeEndStr": "2025-12-03T08:30:00Z"
      },
      "rootCause": {
        "service": "auth-service",
        "component": "token_validator",
        "evidence": [
          {
            "type": "correlation",
            "details": "Spearman correlation: 0.92, p-value: 0.001"
          },
          {
            "type": "temporal",
            "details": "auth errors led payment failures by 2 minutes"
          }
        ]
      },
      "chains": [
        {
          "score": 0.89,
          "durationHops": 2,
          "steps": [
            {
              "kpiName": "auth_token_errors_total",
              "service": "auth-service",
              "suspicionScore": 0.92
            },
            {
              "kpiName": "payment_failures_total",
              "service": "payment-service",
              "suspicionScore": 0.85
            }
          ]
        }
      ],
      "diagnostics": {
        "timeRings": {
          "strategy": "adaptive",
          "definitions": [
            {
              "ringName": "R0",
              "relativeStart": "-15m",
              "relativeEnd": "-5m"
            },
            {
              "ringName": "R1",
              "relativeStart": "-5m",
              "relativeEnd": "+5m"
            }
          ]
        }
      }
    }
  }
}
```

### Response Format

```json
{
  "status": "success",
  "data": {
    "explanation": "Between 7:30 AM and 8:30 AM UTC, the payment service experienced significant failures affecting customer transactions. The root cause was traced to authentication token validation errors in the auth service.\n\nThe incident began when the authentication service started rejecting valid tokens approximately 2 minutes before payment failures spiked. This cascading failure meant that legitimate payment requests were denied due to authentication issues, not actual payment processing problems.\n\nKey evidence:\n- Strong statistical correlation (92% confidence) between auth errors and payment failures\n- Auth errors consistently preceded payment failures by 2 minutes\n- 85% severity rating indicating substantial business impact\n\nRecommended actions:\n- Investigate auth service token validator component\n- Review token expiration policies\n- Implement better circuit breakers to prevent cascading failures",
    "tokensUsed": 458,
    "provider": "openai",
    "model": "gpt-4",
    "generatedAt": "2025-12-03T14:45:23Z",
    "cached": false,
    "generationTimeMs": 2341
  }
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `explanation` | string | Non-technical narrative explanation of the RCA |
| `tokensUsed` | integer | Number of AI tokens consumed (for cost tracking) |
| `provider` | string | AI provider used (openai, anthropic, ollama, vllm) |
| `model` | string | Specific model used (e.g., gpt-4, claude-3-5-sonnet) |
| `generatedAt` | string | ISO 8601 timestamp of explanation generation |
| `cached` | boolean | Whether response was served from cache |
| `generationTimeMs` | integer | Time taken to generate explanation (milliseconds) |

### Error Responses

#### 400 Bad Request
```json
{
  "status": "error",
  "error": "invalid_json_payload"
}
```

Returned when:
- Request body is not valid JSON
- Required field `rcaData` is missing
- RCA data structure is invalid

#### 429 Too Many Requests
```json
{
  "status": "error",
  "error": "rate_limit_exceeded",
  "retryAfter": 45
}
```

Returned when rate limit is exceeded. Default limit: 10 requests per minute per client.

#### 500 Internal Server Error
```json
{
  "status": "error",
  "error": "mira_generation_failed"
}
```

Returned when:
- AI provider is unavailable
- TOON conversion fails
- Prompt template rendering fails

## Configuration

MIRA is configured via the `mira` section in `configs/config.yaml`:

```yaml
mira:
  enabled: true
  provider: "openai"  # openai | anthropic | ollama | vllm
  
  # Rate limiting
  rateLimit:
    enabled: true
    requestsPerMinute: 10
  
  # Caching
  cache:
    enabled: true
    ttl: 3600  # 1 hour
  
  # Timeout for AI requests
  timeout: 30s
  
  # Provider configurations
  openai:
    apiKey: "${OPENAI_API_KEY}"
    model: "gpt-4"
    maxTokens: 1500
    temperature: 0.7
  
  anthropic:
    apiKey: "${ANTHROPIC_API_KEY}"
    model: "claude-3-5-sonnet-20241022"
    maxTokens: 1500
    temperature: 0.7
  
  ollama:
    baseURL: "http://localhost:11434"
    model: "llama3.1:70b"
    maxTokens: 1500
    temperature: 0.7
  
  vllm:
    baseURL: "http://localhost:8000"
    model: "meta-llama/Llama-3.1-70B-Instruct"
    maxTokens: 1500
    temperature: 0.7
  
  # Prompt template (Go text/template syntax)
  promptTemplate: |
    You are a technical incident analyst explaining a root cause analysis to a non-technical business audience.
    
    Translate the following technical RCA data into a clear, concise narrative suitable for executives and business stakeholders.
    
    Time window: {{.TimeWindowStart}} to {{.TimeWindowEnd}}
    
    Impact: {{.ImpactService}} - {{.MetricName}} (severity: {{.Severity}})
    Root Cause: {{.RootCauseService}} - {{.RootCauseComponent}}
    Evidence: {{.RootCauseEvidence}}
    
    Causal Chain: {{.TopChainPath}} (confidence: {{.TopChainScore}})
    
    Provide:
    1. A brief summary of what happened
    2. The root cause in plain language
    3. Why this root cause led to the observed impact
    4. Recommended actions
    
    Keep the explanation under 300 words. Avoid jargon, acronyms, and technical metrics.
```

### Environment Variables

API keys are loaded from environment variables for security:

```bash
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
```

## Provider Comparison

| Provider | Pros | Cons | Best For |
|----------|------|------|----------|
| **OpenAI** | High quality, fast, reliable | Costly, cloud dependency | Production use, critical incidents |
| **Anthropic** | Excellent reasoning, long context | Expensive, cloud dependency | Complex RCA chains, detailed analysis |
| **Ollama** | Free, private, low latency | Requires local setup, GPU recommended | Development, privacy-sensitive environments |
| **vLLM** | Self-hosted, customizable, fast | Infrastructure overhead, GPU required | Large scale, custom models |

### Cost Considerations

**OpenAI GPT-4** (~$0.03-0.06 per request):
- Input: $0.03 per 1K tokens (~400 tokens per RCA)
- Output: $0.06 per 1K tokens (~300 tokens per explanation)
- Average cost: **$0.03** per explanation

**Anthropic Claude** (~$0.015-0.075 per request):
- Input: $0.003 per 1K tokens
- Output: $0.015 per 1K tokens
- Average cost: **$0.015** per explanation

**Ollama/vLLM** (infrastructure only):
- No per-request cost
- GPU instance: ~$1-3/hour for A100
- Cost-effective for >100 requests/day

## TOON Format

MIRA uses **TOON (Token Oriented Object Notation)** to convert RCA JSON into a more LLM-friendly format. TOON reduces token count by 30-60% compared to raw JSON.

**JSON** (verbose):
```json
{
  "impact": {
    "impactService": "payment-service",
    "metricName": "payment_failures_total",
    "severity": 0.85
  }
}
```

**TOON** (compact):
```
impact:
  impactService: payment-service
  metricName: payment_failures_total
  severity: 0.85
```

TOON benefits:
- **Token efficiency**: 30-60% fewer tokens
- **Human-readable**: Easier for LLMs to parse
- **Preserves structure**: All data retained, just formatted better

## Caching Strategy

MIRA implements two-tier caching:

### 1. Valkey In-Memory Cache (Fast)
- **TTL**: 1 hour (configurable)
- **Key**: SHA256 hash of rendered prompt
- **Hit Rate**: ~70-80% for repeated incidents
- **Latency**: <10ms

### 2. Weaviate Vector Cache (Semantic) - *Future Enhancement*
- **Purpose**: Find similar incidents even with different wording
- **Use Case**: "Payment failures due to auth errors" vs "Auth service caused payment issues"
- **Status**: Deferred to future release

Cache invalidation:
- Automatic after TTL expiration
- Manual via cache flush (admin endpoint)
- Per-provider caching (different providers = different cache keys)

## Rate Limiting

MIRA has dedicated rate limiting to prevent runaway costs:

### Configuration
```yaml
mira:
  rateLimit:
    enabled: true
    requestsPerMinute: 10  # Conservative default
```

### Rate Limit Headers

Every response includes rate limit headers:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
X-RateLimit-Reset: 1701615600
```

### Exceeding Limits

When rate limit is exceeded:
```json
HTTP/1.1 429 Too Many Requests
Retry-After: 45

{
  "status": "error",
  "error": "rate_limit_exceeded"
}
```

## Usage Examples

### cURL

```bash
# 1. Get RCA data
RCA_RESPONSE=$(curl -X POST http://localhost:8010/api/v1/unified/rca \
  -H "Content-Type: application/json" \
  -d '{
    "startTime": "2025-12-03T07:30:00Z",
    "endTime": "2025-12-03T08:30:00Z"
  }')

# 2. Get MIRA explanation
curl -X POST http://localhost:8010/api/v1/mira/rca_analyze \
  -H "Content-Type: application/json" \
  -d "{\"rcaData\": $RCA_RESPONSE}"
```

### Python

```python
import requests
from datetime import datetime, timedelta

# 1. Get RCA data
end_time = datetime.utcnow()
start_time = end_time - timedelta(hours=1)

rca_response = requests.post(
    "http://localhost:8010/api/v1/unified/rca",
    json={
        "startTime": start_time.isoformat() + "Z",
        "endTime": end_time.isoformat() + "Z"
    }
).json()

# 2. Get MIRA explanation
mira_response = requests.post(
    "http://localhost:8010/api/v1/mira/rca_analyze",
    json={"rcaData": rca_response}
)

explanation = mira_response.json()["data"]["explanation"]
print(f"Explanation:\n{explanation}")
print(f"\nCached: {mira_response.json()['data']['cached']}")
print(f"Tokens: {mira_response.json()['data']['tokensUsed']}")
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type RCARequest struct {
    StartTime string `json:"startTime"`
    EndTime   string `json:"endTime"`
}

type MIRARequest struct {
    RCAData json.RawMessage `json:"rcaData"`
}

func main() {
    // 1. Get RCA data
    rcaReq := RCARequest{
        StartTime: time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
        EndTime:   time.Now().UTC().Format(time.RFC3339),
    }
    
    rcaBody, _ := json.Marshal(rcaReq)
    rcaResp, _ := http.Post(
        "http://localhost:8010/api/v1/unified/rca",
        "application/json",
        bytes.NewBuffer(rcaBody),
    )
    defer rcaResp.Body.Close()
    
    var rcaData json.RawMessage
    json.NewDecoder(rcaResp.Body).Decode(&rcaData)
    
    // 2. Get MIRA explanation
    miraReq := MIRARequest{RCAData: rcaData}
    miraBody, _ := json.Marshal(miraReq)
    
    miraResp, _ := http.Post(
        "http://localhost:8010/api/v1/mira/rca_analyze",
        "application/json",
        bytes.NewBuffer(miraBody),
    )
    defer miraResp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(miraResp.Body).Decode(&result)
    
    data := result["data"].(map[string]interface{})
    fmt.Printf("Explanation:\n%s\n", data["explanation"])
    fmt.Printf("\nCached: %v\n", data["cached"])
    fmt.Printf("Tokens: %.0f\n", data["tokensUsed"])
}
```

## Monitoring & Observability

### Metrics

MIRA exposes Prometheus metrics:

```
# Request counts
mira_rca_analyze_requests_total{provider="openai",status="success"} 1523
mira_rca_analyze_requests_total{provider="openai",status="error"} 12

# Cache performance
mira_rca_analyze_cache_hits_total{provider="openai"} 1089
mira_rca_analyze_cache_misses_total{provider="openai"} 434

# Latency
mira_rca_analyze_ai_latency_seconds{provider="openai",quantile="0.5"} 2.3
mira_rca_analyze_ai_latency_seconds{provider="openai",quantile="0.99"} 8.7

# Token usage
mira_rca_analyze_ai_tokens_used_total{provider="openai",model="gpt-4"} 687234

# Errors
mira_rca_analyze_ai_errors_total{provider="openai",error_type="timeout"} 3
mira_rca_analyze_ai_errors_total{provider="openai",error_type="rate_limit"} 7
```

### Alerts

**High Error Rate:**
```yaml
- alert: MIRAHighErrorRate
  expr: |
    rate(mira_rca_analyze_requests_total{status="error"}[5m]) > 0.05
  for: 5m
  annotations:
    summary: "MIRA error rate above 5%"
```

**Low Cache Hit Rate:**
```yaml
- alert: MIRALowCacheHitRate
  expr: |
    rate(mira_rca_analyze_cache_hits_total[10m]) /
    (rate(mira_rca_analyze_cache_hits_total[10m]) + 
     rate(mira_rca_analyze_cache_misses_total[10m])) < 0.5
  for: 10m
  annotations:
    summary: "MIRA cache hit rate below 50%"
```

**High Latency:**
```yaml
- alert: MIRAHighLatency
  expr: |
    histogram_quantile(0.99, 
      rate(mira_rca_analyze_ai_latency_seconds_bucket[5m])) > 10
  for: 5m
  annotations:
    summary: "MIRA p99 latency above 10 seconds"
```

## Troubleshooting

### Issue: "mira_generation_failed" Errors

**Possible Causes:**
1. AI provider API key invalid/expired
2. Network connectivity issues
3. Provider rate limits exceeded
4. Model not available

**Resolution:**
```bash
# Check API key
echo $OPENAI_API_KEY

# Test provider connectivity
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# Check logs
kubectl logs -l app=mirador-core --tail=100 | grep MIRA
```

### Issue: High Token Costs

**Mitigation Strategies:**

1. **Enable Caching:**
```yaml
mira:
  cache:
    enabled: true
    ttl: 7200  # Increase to 2 hours
```

2. **Use Cheaper Models:**
```yaml
mira:
  openai:
    model: "gpt-3.5-turbo"  # ~10x cheaper than gpt-4
```

3. **Switch to Self-Hosted:**
```yaml
mira:
  provider: "ollama"  # No per-request cost
```

4. **Optimize Prompt Template:**
```yaml
# Reduce input tokens by removing unnecessary context
promptTemplate: |
  Explain RCA: {{.RootCauseService}} caused {{.ImpactService}} failure.
  Time: {{.TimeWindowStart}} to {{.TimeWindowEnd}}
  Evidence: {{.RootCauseEvidence}}
  
  Keep under 200 words, plain language.
```

### Issue: Slow Response Times

**Diagnosis:**
```bash
# Check if cached
curl -X POST http://localhost:8010/api/v1/mira/rca_analyze \
  -H "Content-Type: application/json" \
  -d @rca_payload.json | jq '.data.cached'

# Check generation time
curl -X POST http://localhost:8010/api/v1/mira/rca_analyze \
  -H "Content-Type: application/json" \
  -d @rca_payload.json | jq '.data.generationTimeMs'
```

**Optimization:**
1. Increase cache TTL for better cache hit rate
2. Switch to faster model (gpt-3.5-turbo vs gpt-4)
3. Reduce `maxTokens` in config
4. Use local Ollama for sub-second responses

### Issue: Rate Limit Exceeded

**Quick Fix:**
```yaml
mira:
  rateLimit:
    requestsPerMinute: 30  # Increase limit
```

**Long-Term:**
1. Implement request queuing on client side
2. Use exponential backoff for retries
3. Pre-cache explanations for known incident patterns

## Best Practices

### 1. Prompt Engineering

**Good Prompt Template:**
```yaml
promptTemplate: |
  Explain this incident to a CEO who has 2 minutes.
  
  What broke: {{.ImpactService}}
  Why it broke: {{.RootCauseService}}
  When: {{.TimeWindowStart}} to {{.TimeWindowEnd}}
  Confidence: {{.TopChainScore}}
  
  Format:
  - One sentence summary
  - Root cause in plain English
  - Business impact
  - Next steps
  
  No jargon. Under 150 words.
```

**Bad Prompt Template:**
```yaml
promptTemplate: |
  Here's the TOON data: {{.TOONData}}
  
  Explain it.
```

### 2. Cost Optimization

1. **Always enable caching** - 70%+ cache hit rate in production
2. **Start with gpt-3.5-turbo** - Upgrade to gpt-4 only if quality issues
3. **Use Ollama for dev** - Zero marginal cost during development
4. **Monitor token usage** - Set up alerts at $100/month threshold

### 3. Quality Assurance

1. **Validate explanations** - Spot check against technical RCA
2. **A/B test prompts** - Compare explanation quality across templates
3. **Collect feedback** - Add thumbs up/down for explanations
4. **Version prompts** - Track prompt template changes in git

### 4. Security

1. **Never log API keys** - Use environment variables, not config files
2. **Rotate keys regularly** - Especially if logs exposed
3. **Rate limit aggressively** - Prevent API key theft impact
4. **Audit access** - Track who calls MIRA endpoint

## Related Documentation

- [RCA Engine Architecture](rca.md)
- [Correlation Engine Guide](correlation.md)
- [Unified Query API](unified-query.md)
- [Configuration Guide](configuration.md)
- [Monitoring & Observability](monitoring-observability.md)

## Support & Feedback

For issues or questions:
- GitHub Issues: [platformbuilds/mirador-core/issues](https://github.com/platformbuilds/mirador-core/issues)
- Email: support@platformbuilds.org
- Documentation: [mirador-core.readthedocs.io](https://mirador-core.readthedocs.io)
