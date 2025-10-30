#!/usr/bin/env python3
import sys, json
try:
    import yaml
except Exception as e:
    print('PyYAML not installed. Install via pip install pyyaml', file=sys.stderr)
    sys.exit(1)
from pathlib import Path
y = Path('api/openapi.yaml').read_text(encoding='utf-8')
data = yaml.safe_load(y)
assert isinstance(data, dict) and 'openapi' in data, 'Invalid OpenAPI YAML: missing openapi key'
print('YAML parse OK; version:', data.get('openapi'))
print('Paths count:', len((data.get('paths') or {})))
print('Components:', 'schemas' in (data.get('components') or {}))
print('Validation (structural) OK')
