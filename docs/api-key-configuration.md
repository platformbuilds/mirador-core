# API Key Configuration Guide

This document explains how to configure API key limits in Mirador Core through Helm charts (Kubernetes) and configuration files (non-Kubernetes environments).

## Overview

The API key limits system is designed to be highly configurable and supports:
- **Default system-wide limits** for all tenants
- **Tenant-specific overrides** for custom limits per tenant  
- **Global administrative overrides** that apply across all tenants
- **Flexible permission controls** for admin-managed updates
- **Expiry constraints** to enforce security policies

## Configuration Structure

### Helm Chart Configuration (Kubernetes)

Add the following to your `values.yaml`:

```yaml
mirador:
  # API Key Management Configuration
  api_keys:
    enabled: true
    # Default limits applied system-wide unless overridden
    default_limits:
      max_keys_per_user: 10
      max_keys_per_tenant_admin: 25
      max_keys_per_global_admin: 100
    # Tenant-specific limit overrides
    tenant_limits:
      - tenant_id: "production"
        max_keys_per_user: 20
        max_keys_per_tenant_admin: 50
        max_keys_per_global_admin: 150
      - tenant_id: "development"
        max_keys_per_user: 5
        max_keys_per_tenant_admin: 15
        max_keys_per_global_admin: 50
    # Global system-wide overrides (optional)
    global_limits_override:
      max_keys_per_user: 15
      max_keys_per_tenant_admin: 35
      max_keys_per_global_admin: 200
      max_total_keys: 5000
    # Permission settings
    allow_tenant_override: true   # Allow tenant admins to override limits
    allow_admin_override: true    # Allow global admins to override limits
    # Expiry settings
    max_expiry_days: 365         # Maximum days for API key expiry
    min_expiry_days: 1           # Minimum days for API key expiry
    enforce_expiry: false        # Require expiry date on all new keys
```

### Configuration File (Non-Kubernetes)

Add to `configs/config.yaml`, `configs/config.development.yaml`, or `configs/config.production.yaml`:

```yaml
# API Key Management Configuration
api_keys:
  enabled: true
  default_limits:
    max_keys_per_user: 10
    max_keys_per_tenant_admin: 25
    max_keys_per_global_admin: 100
  tenant_limits: []  # Can be populated with tenant-specific limits
  global_limits_override: null  # Can override default limits globally
  allow_tenant_override: true
  allow_admin_override: true
  max_expiry_days: 365
  min_expiry_days: 1
  enforce_expiry: false
```

## Environment-Specific Defaults

### Development Environment
- **More permissive limits** for testing (15/35/150 keys)
- **Shorter maximum expiry** (90 days)
- **Tenant overrides enabled** for flexibility
- **Expiry not enforced** for easier testing

### Production Environment  
- **More restrictive limits** for security (5/15/50 keys)
- **Longer maximum expiry** (2 years)
- **Tenant overrides disabled** for security
- **Expiry enforced** on all keys

## Configuration Priority

The system applies limits in the following priority order:

1. **Global Limits Override** (highest priority)
2. **Tenant-Specific Limits** 
3. **Default System Limits** (lowest priority)

## Permission Model

### API Key Permissions
- `apikey.manage` - Create, list, and revoke own API keys
- `apikey.admin` - Manage API key limits for tenant
- `apikey.global_admin` - View system configuration and global overrides

### Configuration Access
- **Regular Users**: Can view their own limits via API
- **Tenant Admins**: Can update tenant limits (if `allow_tenant_override: true`)
- **Global Admins**: Can view system configuration and update all limits

## API Endpoints

### Get API Key Limits
```text
GET /api/v1/auth/apikey-limits
Authorization: Bearer <jwt_token>
```

Returns current limits for the authenticated user's tenant, including configuration metadata.

### Update API Key Limits (Admin Only)
```text
PUT /api/v1/auth/apikey-limits
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
  "maxKeysPerUser": 15,
  "maxKeysPerTenantAdmin": 30,
  "maxKeysPerGlobalAdmin": 120
}
```

### Get System Configuration (Global Admin Only)
```text
GET /api/v1/auth/apikey-config
Authorization: Bearer <jwt_token>
```

Returns system-wide configuration including all settings and their sources.

## Security Considerations

### Configuration Security
- Configuration files should be secured with appropriate file permissions
- Helm chart values should use Kubernetes secrets for sensitive settings
- Consider using ConfigMaps for non-sensitive configuration updates

### Runtime Protection
- `allow_tenant_override: false` prevents tenant admins from bypassing limits
- `allow_admin_override: false` makes limits completely immutable at runtime
- `enforce_expiry: true` ensures all API keys have expiration dates

### Monitoring
- All limit changes are logged with user attribution
- API key generation attempts against limits are logged
- Configuration access is audited

## Examples

### High-Security Production Setup
```yaml
api_keys:
  enabled: true
  default_limits:
    max_keys_per_user: 3
    max_keys_per_tenant_admin: 10
    max_keys_per_global_admin: 25
  tenant_limits: []
  global_limits_override: null
  allow_tenant_override: false  # No runtime overrides
  allow_admin_override: false   # Immutable limits
  max_expiry_days: 90           # Short expiry
  min_expiry_days: 7
  enforce_expiry: true          # Required expiry
```

### Development/Testing Setup
```yaml
api_keys:
  enabled: true
  default_limits:
    max_keys_per_user: 50
    max_keys_per_tenant_admin: 100
    max_keys_per_global_admin: 200
  tenant_limits: []
  global_limits_override: null
  allow_tenant_override: true   # Flexible for testing
  allow_admin_override: true
  max_expiry_days: 0            # No limit (0 = unlimited)
  min_expiry_days: 0
  enforce_expiry: false         # Optional expiry
```

### Multi-Tenant Production
```yaml
api_keys:
  enabled: true
  default_limits:
    max_keys_per_user: 5
    max_keys_per_tenant_admin: 15
    max_keys_per_global_admin: 50
  tenant_limits:
    - tenant_id: "enterprise_client_1"
      max_keys_per_user: 25
      max_keys_per_tenant_admin: 75
      max_keys_per_global_admin: 150
    - tenant_id: "startup_client_2"  
      max_keys_per_user: 3
      max_keys_per_tenant_admin: 8
      max_keys_per_global_admin: 20
  global_limits_override:
    max_total_keys: 10000  # System-wide cap
  allow_tenant_override: true
  allow_admin_override: true
  max_expiry_days: 365
  min_expiry_days: 30
  enforce_expiry: true
```

## Troubleshooting

### Common Issues

1. **"API key limit exceeded"**
   - Check current limits: `GET /api/v1/auth/apikey-limits`
   - Verify tenant-specific overrides
   - Contact admin to increase limits if needed

2. **"Tenant admin overrides are disabled"**
   - Configuration has `allow_tenant_override: false`
   - Only global admins can modify limits
   - May require Helm chart update in Kubernetes

3. **"API key expiry validation failed"**
   - Check `min_expiry_days` and `max_expiry_days` settings
   - Verify if `enforce_expiry: true` requires expiration date
   - Adjust expiry date to meet constraints

### Checking Current Configuration

Use the global admin endpoint to view effective configuration:

```bash
curl -H "Authorization: Bearer $JWT_TOKEN" \
     http://mirador-api:8010/api/v1/auth/apikey-config
```

### Configuration Validation

The system validates configuration at startup:
- Limits must be positive integers
- `min_expiry_days` ≤ `max_expiry_days`
- Tenant IDs in overrides must be valid strings
- Boolean flags are properly typed

## Migration and Updates

### Updating Helm Charts
```bash
# Update values.yaml with new limits
helm upgrade mirador ./deployments/chart \
  --values custom-values.yaml \
  --namespace mirador

# Changes take effect after pod restart
kubectl rollout restart deployment mirador-core -n mirador
```

### Updating Config Files
```bash
# Edit configuration file
vi configs/config.production.yaml

# Restart application to reload config
systemctl restart mirador-core
# or
kill -HUP $(pgrep mirador-core)  # if hot-reload supported
```

### Zero-Downtime Updates
For production environments, consider:
1. Blue-green deployments with updated configuration
2. Rolling updates with configuration validation
3. Canary deployments to test new limits