#!/bin/bash

# deploy-with-api-key-limits.sh - Example deployment script showing API key limits configuration

set -e

NAMESPACE="mirador"
RELEASE_NAME="mirador-core"
CHART_PATH="./deployments/chart"

echo "🚀 Deploying Mirador Core with custom API key limits..."

# Create temporary values file with API key configuration
cat > /tmp/api-key-values.yaml <<EOF
mirador:
  environment: production
  
  # Custom API Key Limits for this deployment
  api_keys:
    enabled: true
    
    # Conservative defaults for production
    default_limits:
      max_keys_per_user: 5
      max_keys_per_tenant_admin: 15
      max_keys_per_global_admin: 50
    
    # Tenant-specific overrides
    tenant_limits:
      - tenant_id: "enterprise-customer-1"
        max_keys_per_user: 25
        max_keys_per_tenant_admin: 75
        max_keys_per_global_admin: 150
      - tenant_id: "startup-customer-2"
        max_keys_per_user: 3
        max_keys_per_tenant_admin: 8
        max_keys_per_global_admin: 20
      - tenant_id: "internal-testing"
        max_keys_per_user: 50
        max_keys_per_tenant_admin: 100
        max_keys_per_global_admin: 200
    
    # Global safety limits
    global_limits_override:
      max_total_keys: 10000  # System-wide maximum
    
    # Security settings for production
    allow_tenant_override: false  # Prevent runtime modifications
    allow_admin_override: true    # Allow global admins to override
    
    # Strict expiry policies
    enforce_expiry: true          # All keys must have expiry
    min_expiry_days: 7           # Minimum 1 week
    max_expiry_days: 365         # Maximum 1 year

  # Other production settings
  replicaCount: 3
  
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 1000m
      memory: 2Gi

# Weaviate subchart configuration
weaviate:
  enabled: true
  replicas: 2
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 1000m
      memory: 2Gi

# Valkey subchart configuration  
valkey:
  enabled: true
  architecture: replication
  replica:
    replicaCount: 2
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi
EOF

echo "📝 Generated API key configuration:"
echo "---"
cat /tmp/api-key-values.yaml | grep -A 50 "api_keys:"
echo "---"

echo
echo "🔍 Validating Helm chart with new configuration..."
helm template "$RELEASE_NAME" "$CHART_PATH" \
  --namespace "$NAMESPACE" \
  --values /tmp/api-key-values.yaml \
  --dry-run \
  > /dev/null

echo "✅ Helm chart validation successful!"

echo
echo "🚀 Deploying to Kubernetes..."

# Create namespace if it doesn't exist
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# Deploy with Helm
helm upgrade --install "$RELEASE_NAME" "$CHART_PATH" \
  --namespace "$NAMESPACE" \
  --values /tmp/api-key-values.yaml \
  --wait \
  --timeout 10m

echo
echo "✅ Deployment successful!"

echo
echo "📊 Verifying deployment..."
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=mirador-core

echo
echo "🔧 Useful commands:"
echo
echo "# Check pod logs:"
echo "kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=mirador-core --tail=100"
echo
echo "# Port-forward to access API:"
echo "kubectl port-forward -n $NAMESPACE svc/mirador-core 8010:8010"
echo
echo "# Test API key configuration endpoint (requires global admin JWT):"
echo "curl -H \"Authorization: Bearer \$API_KEY\" http://localhost:8010/api/v1/auth/apikey-config"
echo
echo "# Get API key limits for tenant (requires user JWT):"
echo "curl -H \"Authorization: Bearer \$API_KEY\" http://localhost:8010/api/v1/auth/apikey-limits"

# Cleanup temporary file
rm /tmp/api-key-values.yaml

echo
echo "🎉 Mirador Core deployed with custom API key limits!"
echo "📚 See docs/api-key-configuration.md for more configuration options."