# Mirador Core Postman Collection

This directory contains the Postman collection for testing the Mirador Core API endpoints.

## Files

- `mirador-core.postman_collection.json` - Complete Postman collection with all API endpoints
- `openapi.yaml` - OpenAPI 3.x specification in YAML format
- `openapi.json` - OpenAPI 3.x specification in JSON format

## Importing the Collection

1. Open Postman
2. Click "Import" in the top left
3. Select "File" tab
4. Choose `mirador-core.postman_collection.json`
5. Click "Import"

## Collection Variables

The collection includes the following variables that you need to configure:

- `scheme` - Protocol (default: `http`)
- `host` - Server hostname (default: `localhost`)
- `port` - Server port (default: `8010`)
- `apiKey` - Your API key (leave empty if using Bearer auth)
- `bearerToken` - Your JWT bearer token (leave empty if using API key auth)

## Authentication

Mirador Core supports two authentication methods:

1. **API Key Authentication**: Set the `X-API-Key` header with your API key
2. **Bearer Token Authentication**: Set the `Authorization` header with `Bearer <token>`

The collection automatically includes the appropriate headers for endpoints that require authentication.

## Organization

The collection is organized into folders based on API functionality:

- **System** - Health checks and system status
- **Authentication** - Login, logout, token validation, API key management
- **Api Keys** - API key CRUD operations and limits
- **Rbac** - Role-based access control (roles, permissions, groups, role bindings)
- **Kpi Definitions** - KPI definition management
- **Kpi Layouts** - Dashboard layout configuration
- **Dashboards** - Dashboard CRUD operations
- **Metrics** - VictoriaMetrics integration (queries, metadata)
- **Logs** - VictoriaLogs integration (queries, streams, search)
- **Traces** - Trace data queries and flamegraphs
- **Rca** - Root cause analysis operations
- **Config** - Configuration management
- **Sessions** - Session management
- **Tenants** - Multi-tenant management (admin only)
- **Users** - User management (admin only)
- **Ws** - WebSocket streams
- **Unified** - Unified query engine
- **Uql** - Unified Query Language
- **Metrics Metadata** - Metrics metadata discovery and sync

## Usage Tips

1. **Start with Authentication**: Begin by testing the login endpoint to get a session token or API key
2. **Set Variables**: Update the collection variables to match your Mirador Core deployment
3. **Test System Endpoints**: Use the health check endpoints to verify connectivity
4. **Explore Features**: Each folder contains related endpoints for specific functionality
5. **Check Parameters**: Many endpoints have path parameters (shown as `{{param}}`) and query parameters

## Example Workflow

1. Import the collection
2. Set `host` and `port` variables for your deployment
3. Test the `/health` endpoint to verify connectivity
4. Use `/api/v1/auth/login` to authenticate and get tokens
5. Set the `apiKey` or `bearerToken` variable with your credentials
6. Explore other endpoints with proper authentication

## Generating Updated Collections

If the API changes, you can regenerate the Postman collection using:

```bash
python3 tools/gen_postman_collection.py api/openapi.yaml
```

This will update the `mirador-core.postman_collection.json` file with the latest API endpoints.