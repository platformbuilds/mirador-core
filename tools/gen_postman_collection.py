#!/usr/bin/env python3
"""
Generate Postman collection from OpenAPI specification
"""

import yaml
import json
import sys
from pathlib import Path

def load_openapi_spec(spec_path):
    """Load OpenAPI spec from YAML or JSON file"""
    with open(spec_path, 'r') as f:
        if spec_path.suffix.lower() == '.yaml' or spec_path.suffix.lower() == '.yml':
            return yaml.safe_load(f)
        else:
            return json.load(f)

def get_request_body_schema(schema_ref, components):
    """Get schema for request body"""
    if not schema_ref or '$ref' not in schema_ref:
        return None

    ref_path = schema_ref['$ref'].split('/')[-1]
    return components.get('schemas', {}).get(ref_path)

def create_postman_request(path, method, operation, components, base_url_vars):
    """Create a Postman request from OpenAPI operation"""

    # Build URL
    url_parts = []
    query_params = []

    # Handle path parameters
    path_parts = path.split('/')
    for part in path_parts:
        if part.startswith('{') and part.endswith('}'):
            param_name = part[1:-1]
            url_parts.append(f"{{{{{param_name}}}}}")
        else:
            url_parts.append(part)

    # Build raw URL
    scheme = "{{scheme}}"
    host = "{{host}}"
    port = "{{port}}"
    raw_url = f"{scheme}://{host}:{port}{'/'.join(url_parts)}"

    # Handle query parameters
    if 'parameters' in operation:
        for param in operation['parameters']:
            if param.get('in') == 'query':
                query_params.append({
                    "key": param['name'],
                    "value": f"{{{{{param['name']}}}}}" if param.get('required', False) else "",
                    "description": param.get('description', '')
                })

    # Build headers
    headers = []

    # Add auth headers if security is required
    if 'security' in operation:
        for security_req in operation['security']:
            if 'ApiKeyAuth' in security_req:
                headers.append({
                    "key": "X-API-Key",
                    "value": "{{apiKey}}",
                    "type": "text"
                })
            if 'BearerAuth' in security_req:
                headers.append({
                    "key": "Authorization",
                    "value": "Bearer {{bearerToken}}",
                    "type": "text"
                })

    # Handle request body
    body = None
    if 'requestBody' in operation:
        content = operation['requestBody'].get('content', {})
        if 'application/json' in content:
            schema = content['application/json'].get('schema')
            if schema:
                body = {
                    "mode": "raw",
                    "raw": json.dumps({
                        "example": "Replace with actual request data"
                    }, indent=2),
                    "options": {
                        "raw": {
                            "language": "json"
                        }
                    }
                }

    # Create the request
    request = {
        "method": method.upper(),
        "header": headers,
        "url": {
            "raw": raw_url,
            "host": [f"{scheme}://{host}:{port}"],
            "path": [p for p in url_parts if p],
            "query": query_params
        }
    }

    if body:
        request["body"] = body

    return {
        "name": operation.get('summary', f"{method.upper()} {path}"),
        "request": request,
        "response": []
    }

def create_postman_collection(openapi_spec):
    """Create Postman collection from OpenAPI spec"""

    # Group operations by tags
    tag_groups = {}
    paths = openapi_spec.get('paths', {})

    for path, methods in paths.items():
        for method, operation in methods.items():
            if method.lower() not in ['get', 'post', 'put', 'delete', 'patch', 'head', 'options']:
                continue

            tags = operation.get('tags', ['untagged'])
            for tag in tags:
                if tag not in tag_groups:
                    tag_groups[tag] = []
                tag_groups[tag].append((path, method, operation))

    # Create collection items
    items = []
    for tag, operations in tag_groups.items():
        folder_items = []
        for path, method, operation in operations:
            request = create_postman_request(path, method, operation, openapi_spec.get('components', {}), {})
            folder_items.append(request)

        items.append({
            "name": tag.title().replace('-', ' '),
            "item": folder_items
        })

    # Create collection
    collection = {
        "info": {
            "name": openapi_spec['info']['title'],
            "description": openapi_spec['info']['description'],
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
        },
        "item": items,
        "variable": [
            {
                "key": "scheme",
                "value": "http",
                "type": "string"
            },
            {
                "key": "host",
                "value": "localhost",
                "type": "string"
            },
            {
                "key": "port",
                "value": "8010",
                "type": "string"
            },
            {
                "key": "apiKey",
                "value": "",
                "type": "string"
            },
            {
                "key": "bearerToken",
                "value": "",
                "type": "string"
            }
        ]
    }

    return collection

def main():
    if len(sys.argv) != 2:
        print("Usage: python gen_postman_collection.py <openapi_spec_file>")
        sys.exit(1)

    spec_path = Path(sys.argv[1])
    if not spec_path.exists():
        print(f"File not found: {spec_path}")
        sys.exit(1)

    # Load OpenAPI spec
    openapi_spec = load_openapi_spec(spec_path)

    # Generate Postman collection
    collection = create_postman_collection(openapi_spec)

    # Write to file
    output_path = spec_path.parent / "mirador-core.postman_collection.json"
    with open(output_path, 'w') as f:
        json.dump(collection, f, indent=2)

    print(f"Generated Postman collection: {output_path}")

if __name__ == "__main__":
    main()