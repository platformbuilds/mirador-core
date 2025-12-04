#!/bin/bash
# ReadTheDocs Setup Script for Mirador Core
# This script helps configure ReadTheDocs integration

set -e

echo "🔧 ReadTheDocs Setup for Mirador Core v7.0.0"
echo "=========================================="
echo "📊 Including Correlation Engine Documentation (Phase 3 - Completed)"

# Check if we're in the right directory
if [ ! -f ".readthedocs.yaml" ]; then
    echo "❌ Error: .readthedocs.yaml not found. Please run this script from the repository root."
    exit 1
fi

echo "✅ Found .readthedocs.yaml configuration"

# Validate the configuration
echo "🔍 Validating ReadTheDocs configuration..."
python3 -c "
import yaml
import sys
try:
    with open('.readthedocs.yaml', 'r') as f:
        config = yaml.safe_load(f)
    print('✅ YAML syntax is valid')
    if config.get('version') != 2:
        print('⚠️  Warning: Version should be 2')
    if 'sphinx' not in config and 'mkdocs' not in config:
        print('❌ Error: Neither sphinx nor mkdocs configuration found')
        sys.exit(1)
    print('✅ Configuration structure is valid')
except Exception as e:
    print(f'❌ Error parsing YAML: {e}')
    sys.exit(1)
"

# Test documentation build
echo "🔨 Testing documentation build..."
if command -v python3 &> /dev/null; then
    cd docs
    if [ -f "requirements.txt" ]; then
        echo "📦 Installing documentation dependencies..."
        python3 -m pip install -r requirements.txt
    fi

    echo "🏗️  Building documentation..."
    python3 -m sphinx -b html . _build/html --keep-going
    echo "✅ Documentation build successful"
    cd ..
else
    echo "⚠️  Python3 not found, skipping build test"
fi

echo ""
echo "📚 ReadTheDocs Setup Instructions:"
echo "=================================="
echo ""
echo "1. Go to https://readthedocs.org/"
echo "2. Sign in with your GitHub account"
echo "3. Click 'Import a Project'"
echo "4. Connect your GitHub repository: platformbuilds/mirador-core"
echo "5. Configure the project:"
echo "   - Name: Mirador Core"
echo "   - Repository: https://github.com/platformbuilds/mirador-core"
echo "   - Default branch: v7.0.0"
echo "   - Python version: 3.11"
echo "   - Requirements file: docs/requirements.txt"
echo "   - Configuration file: .readthedocs.yaml"
echo ""
echo "6. For webhook integration (optional but recommended):"
echo "   - Go to Admin > Integrations in ReadTheDocs"
echo "   - Add a GitHub webhook"
echo "   - Copy the webhook URL"
echo "   - Add these secrets to your GitHub repository:"
echo "     RTD_WEBHOOK_URL: <webhook-url>"
echo "     RTD_TOKEN: <integration-token>"
echo ""
echo "7. The GitHub Actions workflow (.github/workflows/readthedocs.yml)"
echo "   will automatically trigger builds when documentation changes."
echo ""
echo "8. Your documentation will be available at:"
echo "   https://miradorstack.readthedocs.io/"
echo ""
echo "🎉 Setup complete! ReadTheDocs is ready to build your documentation."