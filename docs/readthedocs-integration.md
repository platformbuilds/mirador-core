# ReadTheDocs Integration

This document explains how ReadTheDocs is integrated with the Mirador Core project for automatic documentation publishing.

## Overview

ReadTheDocs automatically builds and hosts the project documentation using Sphinx. The documentation includes:

- API Reference (OpenAPI/Swagger)
- User Guides
- Configuration Documentation
- Deployment Guides
- Migration Guide (including Weaviate Schema Management)
- Correlation Engine Documentation (Phase 3 - Completed)
- Unified Query Architecture

## Configuration Files

### `.readthedocs.yaml`
The main configuration file that tells ReadTheDocs how to build the documentation:

- **Python Version**: 3.11
- **Build Tool**: Sphinx
- **Configuration**: `docs/conf.py`
- **Requirements**: `docs/requirements.txt`
- **Source Directory**: `docs/`

### `docs/requirements.txt`
Python dependencies required for building documentation:

```
sphinx>=5.0.0
sphinx-rtd-theme>=1.2.0
myst-parser>=0.18.0
sphinxext-opengraph>=0.8.0
sphinx-copybutton>=0.5.0
```

### `docs/conf.py`
Sphinx configuration with MyST parser for Markdown support and ReadTheDocs theme.

## GitHub Actions Integration

### ReadTheDocs Workflow (`.github/workflows/readthedocs.yml`)

Triggers documentation builds when:
- Documentation files change (`docs/**`)
- ReadTheDocs configuration changes (`.readthedocs.yaml`)
- OpenAPI specifications change (`api/openapi.*`)

Features:
- **Webhook Integration**: Triggers ReadTheDocs builds via webhook (if configured)
- **Local Validation**: Validates documentation builds locally
- **Artifact Upload**: Uploads build artifacts for debugging

### CI Documentation Validation (`.github/workflows/ci.yml`)

The main CI pipeline includes documentation validation:
- Validates Sphinx builds succeed
- Checks for broken links
- Uploads documentation artifacts

## Setup Instructions

### 1. ReadTheDocs Project Setup

1. Go to [ReadTheDocs](https://readthedocs.org/)
2. Sign in with GitHub
3. Click "Import a Project"
4. Connect repository: `platformbuilds/mirador-core`
5. Configure:
   - **Name**: Mirador Core
   - **Repository**: `https://github.com/platformbuilds/mirador-core`
   - **Default Branch**: `v7.0.0`
   - **Python Version**: 3.11
   - **Requirements File**: `docs/requirements.txt`
   - **Configuration File**: `.readthedocs.yaml`

### 2. Webhook Integration (Recommended)

For automatic builds on every push:

1. In ReadTheDocs: Admin → Integrations → Add Integration
2. Select "GitHub incoming webhook"
3. Copy the webhook URL and token
4. Add to GitHub repository secrets:
   - `RTD_WEBHOOK_URL`: The webhook URL
   - `RTD_TOKEN`: The integration token

### 3. Alternative: Manual Builds

If webhooks aren't configured, ReadTheDocs can be set to:
- Build on every commit (Admin → Advanced Settings)
- Or trigger builds manually from the dashboard

## Documentation URLs

- **Main Documentation**: https://miradorstack.readthedocs.io/
- **API Documentation**: https://mirador-core.github.io/api/
- **Version-specific**: https://miradorstack.readthedocs.io/en/v7.0.0/

## Local Development

To build documentation locally:

```bash
# Install dependencies
pip install -r docs/requirements.txt

# Build HTML documentation
cd docs
sphinx-build -b html . _build/html

# Serve locally (optional)
python -m http.server 8000 -d _build/html
```

## Troubleshooting

### Build Failures

1. Check the GitHub Actions logs for the ReadTheDocs workflow
2. Download build artifacts to inspect errors
3. Validate locally using the commands above

### Common Issues

- **Missing dependencies**: Ensure `docs/requirements.txt` includes all required packages
- **Markdown parsing errors**: Check MyST parser configuration in `docs/conf.py`
- **Broken links**: Run `sphinx-build -b linkcheck` to identify broken references

### Configuration Validation

Use the setup script to validate configuration:

```bash
./tools/setup-readthedocs.sh
```

This script validates:
- YAML syntax
- Configuration structure
- Local documentation build
- Provides setup instructions

## Version Management

ReadTheDocs supports multiple versions:
- **Latest**: Points to default branch (`v7.0.0`)
- **Stable**: Can be configured to point to releases
- **Tags**: Automatic versions for git tags

Configure version management in ReadTheDocs Admin → Versions.