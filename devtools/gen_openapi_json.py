#!/usr/bin/env python3
import sys, json, os

try:
    import yaml  # PyYAML
except Exception:
    sys.stderr.write("PyYAML is not installed. Please install it (e.g., pip install pyyaml) and rerun.\n")
    sys.exit(1)

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
yaml_path = os.path.join(ROOT, 'api', 'openapi.yaml')
json_path = os.path.join(ROOT, 'api', 'openapi.json')

with open(yaml_path, 'r', encoding='utf-8') as f:
    data = yaml.safe_load(f)

with open(json_path, 'w', encoding='utf-8') as f:
    json.dump(data, f, indent=2, ensure_ascii=False, default=str)

print(f"Generated {os.path.relpath(json_path, ROOT)} from YAML.")
