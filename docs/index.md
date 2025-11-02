# MIRADOR-CORE Documentation

```{toctree}
:maxdepth: 2
:caption: Contents:

getting-started
unified-query-architecture
uql-language-guide
correlation-engine
correlation-queries-guide
monitoring-observability
testing
readthedocs-integration
api-reference
deployment
configuration
```

## Overview

MIRADOR-CORE is an advanced observability platform that provides unified access to metrics, logs, traces, and correlation analysis across the entire VictoriaMetrics ecosystem.

### Key Features

- **Unified Query API**: Single endpoint for querying metrics, logs, traces, and correlations
- **VictoriaMetrics Integration**: Native support for VictoriaMetrics, VictoriaLogs, and VictoriaTraces
- **AI-Powered Analysis**: Root cause analysis and predictive fracture detection
- **Multi-Engine Search**: Support for both Lucene and Bleve search engines
- **Schema Management**: Comprehensive metadata management for observability data
- **Enterprise Security**: LDAP/AD integration, RBAC, and comprehensive audit logging

### Architecture

MIRADOR-CORE acts as a unified query layer on top of the VictoriaMetrics ecosystem:

```
┌─────────────────┐    ┌─────────────────┐
│   MIRADOR-CORE  │────│  VictoriaLogs   │
│                 │    │  (Logs)         │
│  Query Router   │────│                 │
│  API Gateway    │    └─────────────────┘
│  Schema Store   │
│                 │    ┌─────────────────┐
│  AI Engines     │────│  VictoriaTraces │
│  (RCA/Predict)  │    │  (Traces)       │
└─────────────────┘    └─────────────────┘
         │
         │
    ┌─────────────────┐
    │ VictoriaMetrics │
    │  (Metrics)      │
    └─────────────────┘
```

## Quick Start

1. **Deploy MIRADOR-CORE** using Helm or Docker
2. **Configure data sources** (VictoriaMetrics ecosystem endpoints)
3. **Set up authentication** (LDAP/AD or OAuth)
4. **Start querying** using the Unified Query API

## Support

- **Documentation**: https://miradorstack.readthedocs.io/
- **API Reference**: https://mirador-core.github.io/api/
- **GitHub Issues**: Bug reports and feature requests
- **Community Forum**: General questions and community help

---

```{rubric} Indices and tables
```

* {ref}`genindex`
* {ref}`modindex`
* {ref}`search`