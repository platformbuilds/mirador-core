# Mirador-Core Multi-Tenant Architecture Strategy

**Version:** 1.0  
**Date:** November 8, 2025  
**Author:** Senior Architecture Team

---

## Executive Summary

This document outlines a comprehensive multi-tenant strategy for Mirador-Core, including:
- Complete tenant isolation across VictoriaMetrics, VictoriaLogs, and VictoriaTraces
- Enhanced data models and middleware for tenant context propagation
- CRUD operations with tenant awareness
- Security and compliance considerations
- Implementation roadmap with code examples

---

## Table of Contents

1. [Current State Analysis](#1-current-state-analysis)
2. [Multi-Tenant Architecture Design](#2-multi-tenant-architecture-design)
3. [Tenant Data Model](#3-tenant-data-model)
4. [Tenant Context Propagation](#4-tenant-context-propagation)
5. [Victoria* Services Integration](#5-victoria-services-integration)
6. [Repository Layer Enhancement](#6-repository-layer-enhancement)
7. [API Layer Updates](#7-api-layer-updates)
8. [Security & Compliance](#8-security--compliance)
9. [Implementation Plan](#9-implementation-plan)
10. [Code Examples](#10-code-examples)

---

## 1. Current State Analysis

### 1.1 Existing Tenant Support

**Current Implementation:**
- ✅ Basic `TenantID` field in models (`CorrelationEvent`, `UserSession`, query requests)
- ✅ Middleware extracts `tenant_id` from session and sets in Gin context
- ✅ VictoriaMetrics/Logs/Traces services pass `TenantID` via `AccountID` header
- ✅ KPI/Schema repositories accept `tenantID` parameter

**Gaps Identified:**
- ❌ No centralized Tenant entity/model with metadata
- ❌ No tenant lifecycle management (create/update/delete/suspend)
- ❌ Inconsistent tenant ID propagation across all services
- ❌ No tenant-level quotas, rate limiting, or resource isolation
- ❌ Limited tenant validation and authorization
- ❌ No tenant-aware caching strategies
- ❌ Missing tenant audit trails and compliance features

### 1.2 Victoria* Services Current State

**VictoriaMetrics:**
- Supports multiple endpoints configuration
- Has multi-endpoint aggregation capability
- Can handle dynamic endpoint routing

**VictoriaLogs:**
- Supports multiple endpoints configuration
- Has multi-source aggregation capability
- Can handle dynamic endpoint routing

**VictoriaTraces:**
- Basic endpoint configuration support
- Needs enhancement for dynamic routing
- Will follow similar pattern to Metrics/Logs

**New Approach - Per-Tenant Deployments:**
Instead of using AccountID headers with shared deployments, we'll maintain separate Victoria* deployments per tenant and route requests intelligently.

---

## 2. Multi-Tenant Architecture Design

### 2.1 Isolation Strategy

**Recommended Approach: Physical Isolation with Separate Deployments**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         MIRADOR-CORE API                                    │
│              (Intelligent Multi-Cluster Routing Layer)                      │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                    │
│  │   Tenant A   │  │   Tenant B   │  │   Tenant C   │                    │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                    │
│         │                  │                  │                             │
│    ┌────▼──────────────────▼──────────────────▼────┐                      │
│    │   Tenant Context & Multi-Cluster Router        │                      │
│    │  (Extract, Validate, Route to Clusters)        │                      │
│    └────┬──────────────────┬──────────────────┬────┘                      │
│         │                  │                  │                             │
│    ┌────▼─────┐       ┌───▼──────┐      ┌───▼──────┐                     │
│    │ Tenant A │       │ Tenant B │      │ Tenant C │                     │
│    │  Multi-  │       │  Multi-  │      │  Multi-  │                     │
│    │ Cluster  │       │ Cluster  │      │ Cluster  │                     │
│    │ Registry │       │ Registry │      │ Registry │                     │
│    └────┬─────┘       └───┬──────┘      └───┬──────┘                     │
└─────────┼──────────────────┼──────────────────┼──────────────────────────┘
          │                  │                  │
     ┌────┴────┬────────┐    │             ┌────┴────┬────────┐
     │         │        │    │             │         │        │
┌────▼───┐ ┌──▼────┐ ┌─▼────▼──┐    ┌────▼───┐ ┌──▼────┐ ┌─▼──────┐
│Prod    │ │Staging│ │   Dev   │    │Prod    │ │Staging│ │  Dev   │
│Cluster │ │Cluster│ │ Cluster │    │Cluster │ │Cluster│ │Cluster │
│(Tenant │ │(Tenant│ │(Tenant  │    │(Tenant │ │(Tenant│ │(Tenant │
│   A)   │ │   A)  │ │   A)    │    │   C)   │ │   C)  │ │   C)   │
│        │ │       │ │         │    │        │ │       │ │        │
│┌──────┐│ │┌─────┐│ │┌───────┐│    │┌──────┐│ │┌─────┐│ │┌──────┐│
││Metrics││ ││Metr-││ ││Metrics││    ││Metrics││ ││Metr-││ ││Metr- ││
│└──────┘│ │└─────┘│ │└───────┘│    │└──────┘│ │└─────┘│ │└─────┘│
│┌──────┐│ │┌─────┐│ │┌───────┐│    │┌──────┐│ │┌─────┐│ │┌──────┐│
││ Logs  ││ ││Logs ││ ││ Logs  ││    ││ Logs  ││ ││Logs ││ ││Logs  ││
│└──────┘│ │└─────┘│ │└───────┘│    │└──────┘│ │└─────┘│ │└─────┘│
│┌──────┐│ │┌─────┐│ │┌───────┐│    │┌──────┐│ │┌─────┐│ │┌──────┐│
││Traces ││ ││Trace││ ││Traces ││    ││Traces ││ ││Trace││ ││Traces││
│└──────┘│ │└─────┘│ │└───────┘│    │└──────┘│ │└─────┘│ │└─────┘│
└────────┘ └───────┘ └─────────┘    └────────┘ └───────┘ └────────┘

K8s Prod    K8s Stag   K8s Dev       K8s Prod   K8s Stag   K8s Dev
Namespace   Namespace  Namespace     Namespace  Namespace  Namespace

**Key Features:**
- Multiple clusters per tenant (prod, staging, dev, custom)
- Independent scaling and configuration per cluster
- Environment-specific isolation and resource allocation
- Flexible cluster targeting via cluster_id or environment tags
```

**Key Principles:**
1. **Multi-Cluster Architecture:** Each tenant can have multiple Victoria* clusters for different purposes
2. **Environment Isolation:** Separate clusters for prod, staging, dev, or custom environments
3. **Physical Isolation:** Each cluster has dedicated Victoria* deployments
4. **No Shared Backend:** Complete data and infrastructure separation between tenants and clusters
5. **Intelligent Multi-Cluster Routing:** Mirador-Core routes requests to the correct tenant cluster based on:
   - Explicit `cluster_id` in request
   - Environment tag (prod/staging/dev)
   - Default cluster configuration
   - Failover priority
6. **Flexible Cluster Selection:** Query any cluster via cluster_id, environment, or tags
7. **Independent Scaling:** Each cluster scales independently based on workload
8. **Security:** Network-level isolation between tenant clusters
9. **Compliance:** Meets strictest data residency and isolation requirements
10. **High Availability:** Multiple clusters enable blue-green deployments and disaster recovery

### 2.2 Tenant Hierarchy

```
Enterprise (Root)
├── Global Roles
│   ├── Global Admin (Root privileges, all tenants + user management)
│   ├── Global Tenant Admin (Tenant administration across platform)
│   └── Tenant User (Standard user, mapped to tenants)
│
├── Users (Global)
│   ├── User 1 (Global Admin)
│   ├── User 2 (Global Tenant Admin)
│   ├── User 3 (Tenant User)
│   └── User 4 (Tenant User)
│
└── Tenants
    ├── Tenant A
    │   ├── User-Tenant Associations
    │   │   ├── User 1 → Tenant Admin (via Global Admin)
    │   │   ├── User 3 → Tenant Admin
    │   │   └── User 4 → Tenant Guest
    │   │
    │   ├── Tenant-Level Roles
    │   │   ├── Tenant Admin (Full tenant control + user mgmt)
    │   │   ├── Tenant Editor (CRUD on resources, no user mgmt)
    │   │   └── Tenant Guest (View-only access)
    │   │
    │   ├── Resources (Dashboards, KPIs, Alerts)
    │   └── Configuration
    │
    └── Tenant B
        ├── User-Tenant Associations
        │   ├── User 1 → Tenant Admin (via Global Admin)
        │   ├── User 2 → Tenant Admin (via Global Tenant Admin)
        │   └── User 3 → Tenant Editor
        │
        ├── Tenant-Level Roles
        │   ├── Tenant Admin
        │   ├── Tenant Editor
        │   └── Tenant Guest
        │
        ├── Resources (Dashboards, KPIs, Alerts)
        └── Configuration
```

**Key Concepts:**
- **Users are global entities** - created once at the Enterprise level
- **Two-tier role system** - Global roles + Tenant-specific roles
- **Global Roles** - Apply across the entire platform
  - **Global Admin**: Root access to everything (all tenants, user management, platform config)
  - **Global Tenant Admin**: Can administer tenants (create/edit/delete tenants)
  - **Tenant User**: Standard user who gets mapped to specific tenants
- **Tenant-Level Roles** - Apply within a specific tenant context
  - **Tenant Admin**: Full control within tenant (user management, resources, config)
  - **Tenant Editor**: Can create/edit resources (dashboards, KPIs) but no user management
  - **Tenant Guest**: Read-only access to tenant resources
- **User-Tenant Associations** - Link users to tenants with tenant-specific roles
- **Same user, different tenant roles** - User 3 can be Tenant Admin in Tenant A and Tenant Editor in Tenant B
- **Resources are tenant-isolated** - dashboards, KPIs, alerts belong to specific tenants

**Benefits of Two-Tier Role Model:**

1. **Clear Separation of Concerns**
   - **Global Roles** handle platform-wide responsibilities
   - **Tenant Roles** handle tenant-specific permissions
   - No confusion about scope of authority

2. **Flexible Role Assignment**
   - Global Admin has automatic Tenant Admin rights in all tenants
   - Global Tenant Admin can manage tenant lifecycle
   - Tenant Users can have different roles per tenant (Admin in Tenant A, Guest in Tenant B)

3. **Simplified Platform Administration**
   - Global Admins manage platform, users, and tenants
   - Global Tenant Admins focus on tenant provisioning and management
   - Tenant Admins focus on their specific tenant operations

4. **Enhanced Security & Governance**
   - Clear privilege escalation path (Guest → Editor → Tenant Admin → Global Tenant Admin → Global Admin)
   - Global roles prevent privilege creep within tenants
   - Audit trail tracks actions at both global and tenant levels

5. **Better User Experience**
   - Users understand their role scope immediately
   - Single login (SSO-friendly) with role-based access
   - Easy tenant switching with appropriate permissions per tenant

6. **Scalability & Multi-Tenant SaaS Ready**
   - Global Admins can create new tenants and assign Global Tenant Admins
   - Tenant Admins manage their own users without platform access
   - Supports B2B SaaS scenarios perfectly

**Role Hierarchy & Permissions:**

```
Global Admin (Highest)
  ├─ Full access to all tenants
  ├─ User management (create, edit, delete all users)
  ├─ Tenant management (create, edit, delete tenants)
  ├─ Platform configuration
  └─ Automatically has Tenant Admin rights in all tenants

Global Tenant Admin
  ├─ Tenant management (create, edit, delete tenants)
  ├─ Assign users to tenants
  ├─ View all tenants
  └─ Cannot manage Global Admins or platform config

Tenant User (Standard User) → Assigned to tenants with:
  ├─ Tenant Admin
  │   ├─ Full control within assigned tenant
  │   ├─ User management within tenant (add/remove users, assign roles)
  │   ├─ Create/edit/delete dashboards, KPIs, alerts
  │   └─ Tenant configuration
  │
  ├─ Tenant Editor
  │   ├─ Create/edit/delete dashboards, KPIs, alerts
  │   ├─ Query metrics, logs, traces
  │   └─ Cannot manage users or tenant settings
  │
  └─ Tenant Guest (Lowest)
      ├─ View-only access to dashboards, KPIs
      ├─ Query metrics, logs, traces (read-only)
      └─ Cannot create or edit anything
```

**Example Scenarios:**

1. **Platform Owner**: aarvee is a **Global Admin** for platformbuilds organization
   - Created and manages the platformbuilds tenant
   - Manages all global users and platform configuration
   - Has full access to all tenants for troubleshooting and support

2. **Platform Owner**: Tony is a **Global Admin** for chikacafe organization
   - Created and manages the chikacafe tenant
   - Manages users and resources for chikacafe
   - Has full access to chikacafe tenant and its resources

3. **Multi-Tenant User**: Akhil is a **Tenant User** with different roles across organizations:
   - **Tenant Guest** in platformbuilds (read-only access to dashboards, metrics, logs)
   - **Tenant Admin** in chikacafe (full administrative control over chikacafe resources)
   - Perfect example of same user with different permissions per tenant

4. **DevOps Engineer**: A **Tenant User** with:
   - **Tenant Admin** in "Development" tenant
   - **Tenant Editor** in "Staging" tenant
   - **Tenant Guest** in "Production" tenant
   - Perfect for promoting changes through environments

5. **External Consultant**: A **Tenant User** with **Tenant Guest** role
   - Read-only access to client dashboards
   - Can query metrics for analysis
   - Cannot make any changes

---

## 3. Tenant Data Model

### 3.1 Core Tenant Model

```go
// internal/models/tenant.go
package models

import (
    "time"
)

// Tenant represents a multi-tenant organization/workspace
type Tenant struct {
    ID          string                 `json:"id" bson:"_id"`
    Name        string                 `json:"name" binding:"required"`
    DisplayName string                 `json:"display_name"`
    Description string                 `json:"description,omitempty"`
    
    // Deployment configuration - separate Victoria* instances
    Deployments TenantDeployments      `json:"deployments" binding:"required"`
    
    // Status management
    Status      TenantStatus           `json:"status"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
    SuspendedAt *time.Time             `json:"suspended_at,omitempty"`
    
    // Contact information
    AdminEmail  string                 `json:"admin_email" binding:"required,email"`
    AdminName   string                 `json:"admin_name"`
    
    // Resource limits and quotas
    Quotas      TenantQuotas           `json:"quotas"`
    
    // Feature flags
    Features    TenantFeatures         `json:"features"`
    
    // Metadata
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    Tags        []string               `json:"tags,omitempty"`
    
    // Billing (optional)
    BillingInfo *BillingInfo           `json:"billing_info,omitempty"`
}

// TenantDeployments holds multiple clusters for tenant-specific Victoria* deployments
type TenantDeployments struct {
    // Multiple VictoriaMetrics clusters (e.g., prod, staging, dev, custom)
    MetricsClusters []ClusterConfig `json:"metrics_clusters" binding:"required,min=1"`
    
    // Multiple VictoriaLogs clusters
    LogsClusters    []ClusterConfig `json:"logs_clusters" binding:"required,min=1"`
    
    // Multiple VictoriaTraces clusters
    TracesClusters  []ClusterConfig `json:"traces_clusters" binding:"required,min=1"`
    
    // Default cluster IDs for each type (if not specified in request)
    DefaultMetricsCluster string `json:"default_metrics_cluster,omitempty"`
    DefaultLogsCluster    string `json:"default_logs_cluster,omitempty"`
    DefaultTracesCluster  string `json:"default_traces_cluster,omitempty"`
}

// ClusterConfig represents a single cluster deployment
type ClusterConfig struct {
    // Unique identifier for this cluster within the tenant
    ClusterID   string `json:"cluster_id" binding:"required"`
    
    // Display name for the cluster
    Name        string `json:"name" binding:"required"`
    
    // Description of cluster purpose
    Description string `json:"description,omitempty"`
    
    // Environment type (prod, staging, dev, qa, custom)
    Environment string `json:"environment" binding:"required"`
    
    // Deployment configuration
    Deployment  DeploymentConfig `json:"deployment" binding:"required"`
    
    // Cluster status
    Status      ClusterStatus `json:"status" default:"active"`
    
    // Priority for failover (higher = preferred)
    Priority    int `json:"priority" default:"0"`
    
    // Tags for flexible cluster selection
    Tags        []string `json:"tags,omitempty"`
    
    // Metadata
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ClusterStatus string

const (
    ClusterStatusActive      ClusterStatus = "active"
    ClusterStatusInactive    ClusterStatus = "inactive"
    ClusterStatusMaintenance ClusterStatus = "maintenance"
    ClusterStatusDegraded    ClusterStatus = "degraded"
)

// DeploymentConfig represents a tenant-specific deployment
type DeploymentConfig struct {
    // Endpoints for this deployment (can be multiple for HA)
    Endpoints []string `json:"endpoints" binding:"required,min=1"`
    
    // Authentication
    Username  string `json:"username,omitempty"`
    Password  string `json:"password,omitempty"`
    
    // Timeout in milliseconds
    Timeout   int    `json:"timeout" default:"30000"`
    
    // Health check configuration
    HealthCheck HealthCheckConfig `json:"health_check,omitempty"`
    
    // Connection pooling
    MaxConnections int `json:"max_connections" default:"100"`
    
    // Deployment metadata
    Namespace   string                 `json:"namespace,omitempty"`   // K8s namespace
    Cluster     string                 `json:"cluster,omitempty"`     // K8s cluster name
    Region      string                 `json:"region,omitempty"`      // Cloud region
    Environment string                 `json:"environment,omitempty"` // dev/staging/prod
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HealthCheckConfig for deployment endpoints
type HealthCheckConfig struct {
    Enabled         bool   `json:"enabled" default:"true"`
    Interval        int    `json:"interval" default:"30"`        // seconds
    Timeout         int    `json:"timeout" default:"5"`          // seconds
    HealthyThreshold   int `json:"healthy_threshold" default:"2"`
    UnhealthyThreshold int `json:"unhealthy_threshold" default:"3"`
    Path            string `json:"path" default:"/health"`
}

type TenantStatus string

const (
    TenantStatusActive    TenantStatus = "active"
    TenantStatusSuspended TenantStatus = "suspended"
    TenantStatusDeleted   TenantStatus = "deleted"
    TenantStatusTrial     TenantStatus = "trial"
)

type TenantQuotas struct {
    // Data retention
    MetricsRetentionDays int `json:"metrics_retention_days"`
    LogsRetentionDays    int `json:"logs_retention_days"`
    TracesRetentionDays  int `json:"traces_retention_days"`
    
    // Rate limits (per minute)
    MaxQueriesPerMinute  int `json:"max_queries_per_minute"`
    MaxIngestRateMB      int `json:"max_ingest_rate_mb"`
    
    // Storage limits
    MaxStorageGB         int `json:"max_storage_gb"`
    
    // User limits
    MaxUsers             int `json:"max_users"`
    MaxDashboards        int `json:"max_dashboards"`
}

type TenantFeatures struct {
    UnifiedQueryEngine  bool `json:"unified_query_engine"`
    AdvancedCorrelation bool `json:"advanced_correlation"`
    AIRootCauseAnalysis bool `json:"ai_root_cause_analysis"`
    CustomIntegrations  bool `json:"custom_integrations"`
    SSO                 bool `json:"sso"`
    AuditLogs           bool `json:"audit_logs"`
}

type BillingInfo struct {
    PlanType       string    `json:"plan_type"` // free, basic, premium, enterprise
    BillingEmail   string    `json:"billing_email"`
    BillingCycle   string    `json:"billing_cycle"` // monthly, annual
    NextBillingDate time.Time `json:"next_billing_date"`
}

// TenantConfig holds tenant-specific configuration
type TenantConfig struct {
    TenantID   string                 `json:"tenant_id"`
    Settings   map[string]interface{} `json:"settings"`
    UpdatedAt  time.Time              `json:"updated_at"`
    UpdatedBy  string                 `json:"updated_by"`
}

// User represents a global user entity (Enterprise-level)
type User struct {
    ID          string                 `json:"id" bson:"_id"`
    Email       string                 `json:"email" binding:"required,email"`
    Username    string                 `json:"username" binding:"required"`
    FullName    string                 `json:"full_name"`
    
    // Global Role Assignment (Platform-level)
    GlobalRole  GlobalRole             `json:"global_role" default:"tenant_user"`
    
    // Authentication
    PasswordHash string                `json:"-"` // Never expose in JSON
    MFAEnabled   bool                  `json:"mfa_enabled"`
    MFASecret    string                `json:"-"` // Never expose in JSON
    
    // Status
    Status       UserStatus            `json:"status"`
    EmailVerified bool                 `json:"email_verified"`
    
    // Timestamps
    CreatedAt    time.Time             `json:"created_at"`
    UpdatedAt    time.Time             `json:"updated_at"`
    LastLoginAt  *time.Time            `json:"last_login_at,omitempty"`
    
    // Profile
    Avatar       string                `json:"avatar,omitempty"`
    Phone        string                `json:"phone,omitempty"`
    Timezone     string                `json:"timezone" default:"UTC"`
    Language     string                `json:"language" default:"en"`
    
    // Metadata
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
    Tags         []string               `json:"tags,omitempty"`
}

type UserStatus string

const (
    UserStatusActive    UserStatus = "active"
    UserStatusInactive  UserStatus = "inactive"
    UserStatusSuspended UserStatus = "suspended"
    UserStatusDeleted   UserStatus = "deleted"
)

// GlobalRole represents platform-level roles
type GlobalRole string

const (
    // GlobalRoleAdmin has root privileges across all tenants and user management
    GlobalRoleAdmin GlobalRole = "global_admin"
    
    // GlobalRoleTenantAdmin can administer tenants (create, edit, delete)
    GlobalRoleTenantAdmin GlobalRole = "global_tenant_admin"
    
    // GlobalRoleTenantUser is a standard user who gets mapped to specific tenants
    GlobalRoleTenantUser GlobalRole = "tenant_user"
)

// GetGlobalPermissions returns platform-level permissions based on global role
func (r GlobalRole) GetGlobalPermissions() []string {
    switch r {
    case GlobalRoleAdmin:
        return []string{
            "platform.admin",
            "users.create", "users.read", "users.update", "users.delete",
            "tenants.create", "tenants.read", "tenants.update", "tenants.delete",
            "tenants.*.admin", // Admin access to all tenants
            "platform.config",
            "audit.read",
        }
    case GlobalRoleTenantAdmin:
        return []string{
            "tenants.create", "tenants.read", "tenants.update", "tenants.delete",
            "users.read", // Can view users to assign them
            "tenants.users.manage", // Can assign users to tenants
        }
    case GlobalRoleTenantUser:
        return []string{
            "users.read.self", // Can read own profile
            "tenants.read", // Can view tenants they have access to
        }
    default:
        return []string{}
    }
}

// TenantUser represents the association between users and tenants
// This is the join table that gives users access to tenants with specific roles
type TenantUser struct {
    ID         string    `json:"id" bson:"_id"`
    TenantID   string    `json:"tenant_id" binding:"required"`
    UserID     string    `json:"user_id" binding:"required"`
    
    // Tenant-Level Role Assignment
    TenantRole TenantRole `json:"tenant_role" binding:"required"`
    
    // Status
    Status     TenantUserStatus `json:"status"`
    
    // Timestamps
    JoinedAt   time.Time `json:"joined_at"`
    UpdatedAt  time.Time `json:"updated_at"`
    RevokedAt  *time.Time `json:"revoked_at,omitempty"`
    
    // Invitation tracking
    InvitedBy  string    `json:"invited_by,omitempty"`
    InvitedAt  time.Time `json:"invited_at,omitempty"`
    
    // Additional permissions override (optional fine-grained control)
    AdditionalPermissions []string `json:"additional_permissions,omitempty"`
    
    // Metadata
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type TenantUserStatus string

const (
    TenantUserStatusActive   TenantUserStatus = "active"
    TenantUserStatusInactive TenantUserStatus = "inactive"
    TenantUserStatusRevoked  TenantUserStatus = "revoked"
)

// TenantRole represents tenant-specific roles
type TenantRole string

const (
    // TenantRoleAdmin has full control within tenant including user management
    TenantRoleAdmin TenantRole = "tenant_admin"
    
    // TenantRoleEditor can create/edit resources but no user management
    TenantRoleEditor TenantRole = "tenant_editor"
    
    // TenantRoleGuest has view-only access (default)
    TenantRoleGuest TenantRole = "tenant_guest"
)

// GetTenantPermissions returns permissions for a tenant role
func (r TenantRole) GetTenantPermissions() []string {
    switch r {
    case TenantRoleAdmin:
        return []string{
            // User management within tenant
            "tenant.users.invite",
            "tenant.users.remove",
            "tenant.users.roles.update",
            "tenant.users.list",
            
            // Resource management
            "dashboards.create", "dashboards.read", "dashboards.update", "dashboards.delete",
            "kpis.create", "kpis.read", "kpis.update", "kpis.delete",
            "alerts.create", "alerts.read", "alerts.update", "alerts.delete",
            
            // Query access
            "metrics.read", "metrics.write",
            "logs.read", "logs.write",
            "traces.read", "traces.write",
            
            // Tenant configuration
            "tenant.config.update",
            "tenant.config.read",
            
            // Roles management
            "roles.create", "roles.read", "roles.update", "roles.delete",
        }
    case TenantRoleEditor:
        return []string{
            // Resource management (no user management)
            "dashboards.create", "dashboards.read", "dashboards.update", "dashboards.delete",
            "kpis.create", "kpis.read", "kpis.update", "kpis.delete",
            "alerts.create", "alerts.read", "alerts.update", "alerts.delete",
            
            // Query access
            "metrics.read", "metrics.write",
            "logs.read", "logs.write",
            "traces.read", "traces.write",
            
            // Read-only config
            "tenant.config.read",
            "roles.read",
        }
    case TenantRoleGuest:
        return []string{
            // View-only access
            "dashboards.read",
            "kpis.read",
            "alerts.read",
            
            // Read-only queries
            "metrics.read",
            "logs.read",
            "traces.read",
            
            // Read-only config
            "tenant.config.read",
        }
    default:
        return []string{}
    }
}

// Role represents a custom tenant-specific role definition (optional advanced feature)
// This allows tenants to define custom roles beyond the built-in ones
type Role struct {
    ID          string   `json:"id" bson:"_id"`
    TenantID    string   `json:"tenant_id" binding:"required"`
    Name        string   `json:"name" binding:"required"`
    DisplayName string   `json:"display_name"`
    Description string   `json:"description,omitempty"`
    
    // Permissions assigned to this role
    Permissions []string `json:"permissions"`
    
    // Role hierarchy
    IsBuiltIn   bool     `json:"is_built_in"` // System-defined vs custom
    IsDefault   bool     `json:"is_default"`  // Assigned to new users by default
    
    // Timestamps
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    CreatedBy   string    `json:"created_by,omitempty"`
}
```

### 3.2 Enhanced UserSession Model

```go
// Update to internal/models/models.go
type UserSession struct {
    ID           string                 `json:"id"`
    UserID       string                 `json:"user_id"`
    
    // Global role (platform-level)
    GlobalRole   GlobalRole             `json:"global_role"`
    GlobalPermissions []string           `json:"global_permissions"`
    
    // Current active tenant context
    TenantID     string                 `json:"tenant_id,omitempty"`
    TenantName   string                 `json:"tenant_name,omitempty"`
    
    // Current tenant's role and permissions
    TenantRole   TenantRole             `json:"tenant_role,omitempty"`
    TenantPermissions []string           `json:"tenant_permissions,omitempty"`
    
    // All accessible tenants for this user
    AccessibleTenants []UserTenantAccess `json:"accessible_tenants"`
    
    // Session management
    CreatedAt    time.Time              `json:"created_at"`
    LastActivity time.Time              `json:"last_activity"`
    ExpiresAt    time.Time              `json:"expires_at"`
    
    // User preferences
    Settings     map[string]interface{} `json:"user_settings"`
    
    // Security tracking
    IPAddress    string                 `json:"ip_address"`
    UserAgent    string                 `json:"user_agent"`
    
    // Token information
    TokenType    string                 `json:"token_type,omitempty"` // "bearer", "api_key"
    TokenID      string                 `json:"token_id,omitempty"`
}

// UserTenantAccess represents a tenant the user has access to
type UserTenantAccess struct {
    TenantID     string     `json:"tenant_id"`
    TenantName   string     `json:"tenant_name"`
    TenantRole   TenantRole `json:"tenant_role"`
    IsDefault    bool       `json:"is_default"` // User's default tenant
    Status       string     `json:"status"`     // "active", "inactive"
}

// HasGlobalPermission checks if user has a specific global permission
func (s *UserSession) HasGlobalPermission(permission string) bool {
    for _, p := range s.GlobalPermissions {
        if p == permission {
            return true
        }
    }
    return false
}

// HasTenantPermission checks if user has a specific permission in current tenant
func (s *UserSession) HasTenantPermission(permission string) bool {
    // Global Admin has all permissions
    if s.GlobalRole == GlobalRoleAdmin {
        return true
    }
    
    for _, p := range s.TenantPermissions {
        if p == permission {
            return true
        }
    }
    return false
}

// IsGlobalAdmin checks if user is a Global Admin
func (s *UserSession) IsGlobalAdmin() bool {
    return s.GlobalRole == GlobalRoleAdmin
}

// IsGlobalTenantAdmin checks if user is a Global Tenant Admin
func (s *UserSession) IsGlobalTenantAdmin() bool {
    return s.GlobalRole == GlobalRoleTenantAdmin || s.GlobalRole == GlobalRoleAdmin
}

// IsTenantAdmin checks if user is admin in current tenant
func (s *UserSession) IsTenantAdmin() bool {
    return s.GlobalRole == GlobalRoleAdmin || s.TenantRole == TenantRoleAdmin
}
```

---

## 4. Tenant Context Propagation

### 4.1 Enhanced Middleware

```go
// internal/api/middleware/tenant.middleware.go
package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// TenantContextMiddleware extracts and validates tenant context
func TenantContextMiddleware(
    tenantRepo repo.TenantRepository,
    tenantUserRepo repo.TenantUserRepository,
    userRepo repo.UserRepository,
    log logger.Logger,
) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract tenant ID from various sources
        tenantID := extractTenantID(c)
        
        if tenantID == "" {
            c.JSON(http.StatusBadRequest, gin.H{
                "status": "error",
                "error":  "Tenant context required",
            })
            c.Abort()
            return
        }
        
        // Validate tenant exists and is active
        tenant, err := tenantRepo.GetTenant(c.Request.Context(), tenantID)
        if err != nil {
            log.Warn("Tenant not found", "tenant_id", tenantID, "error", err)
            c.JSON(http.StatusNotFound, gin.H{
                "status": "error",
                "error":  "Tenant not found",
            })
            c.Abort()
            return
        }
        
        // Check tenant status
        if tenant.Status != models.TenantStatusActive {
            c.JSON(http.StatusForbidden, gin.H{
                "status": "error",
                "error":  "Tenant is not active",
                "tenant_status": string(tenant.Status),
            })
            c.Abort()
            return
        }
        
        // Validate user has access to this tenant
        userID := c.GetString("user_id")
        if userID != "" {
            // Get user to check global role
            user, err := userRepo.GetUser(c.Request.Context(), userID)
            if err != nil {
                c.JSON(http.StatusUnauthorized, gin.H{
                    "status": "error",
                    "error":  "User not found",
                })
                c.Abort()
                return
            }
            
            // Global Admin has automatic access to all tenants
            if user.GlobalRole == models.GlobalRoleAdmin {
                // Set admin context for Global Admin
                c.Set("global_role", user.GlobalRole)
                c.Set("global_permissions", user.GlobalRole.GetGlobalPermissions())
                c.Set("tenant_role", models.TenantRoleAdmin)
                c.Set("tenant_permissions", models.TenantRoleAdmin.GetTenantPermissions())
                c.Set("is_global_admin", true)
            } else {
                // For non-Global Admins, check user-tenant association
                tenantUser, err := tenantUserRepo.GetUserTenantAssociation(
                    c.Request.Context(), 
                    userID, 
                    tenantID,
                )
                if err != nil || tenantUser == nil {
                    c.JSON(http.StatusForbidden, gin.H{
                        "status": "error",
                        "error":  "Access denied to this tenant",
                    })
                    c.Abort()
                    return
                }
                
                // Check association status
                if tenantUser.Status != models.TenantUserStatusActive {
                    c.JSON(http.StatusForbidden, gin.H{
                        "status": "error",
                        "error":  "Tenant access is not active",
                        "user_status": string(tenantUser.Status),
                    })
                    c.Abort()
                    return
                }
                
                // Get permissions for tenant role
                tenantPermissions := tenantUser.TenantRole.GetTenantPermissions()
                
                // Add any additional permissions
                if len(tenantUser.AdditionalPermissions) > 0 {
                    tenantPermissions = append(tenantPermissions, tenantUser.AdditionalPermissions...)
                }
                
                // Set user's context
                c.Set("global_role", user.GlobalRole)
                c.Set("global_permissions", user.GlobalRole.GetGlobalPermissions())
                c.Set("tenant_user", tenantUser)
                c.Set("tenant_role", tenantUser.TenantRole)
                c.Set("tenant_permissions", tenantPermissions)
                c.Set("is_global_admin", false)
            }
        }
        
        // Set tenant context
        c.Set("tenant", tenant)
        c.Set("tenant_id", tenant.ID)
        c.Set("tenant_quotas", tenant.Quotas)
        c.Set("tenant_features", tenant.Features)
        
        // Add tenant info to response headers
        c.Header("X-Tenant-ID", tenant.ID)
        
        c.Next()
    }
}

// extractTenantID gets tenant ID from multiple sources (priority order)
func extractTenantID(c *gin.Context) string {
    // 1. From session (highest priority)
    if tenantID := c.GetString("tenant_id"); tenantID != "" {
        return tenantID
    }
    
    // 2. From custom header
    if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
        return tenantID
    }
    
    // 3. From query parameter
    if tenantID := c.Query("tenant_id"); tenantID != "" {
        return tenantID
    }
    
    // 4. From URL path parameter
    if tenantID := c.Param("tenantId"); tenantID != "" {
        return tenantID
    }
    
    // 5. From request body (for POST/PUT)
    var body struct {
        TenantID string `json:"tenant_id"`
    }
    if err := c.ShouldBindJSON(&body); err == nil && body.TenantID != "" {
        return body.TenantID
    }
    
    return ""
}

// TenantSwitchMiddleware allows switching tenant context
func TenantSwitchMiddleware(
    tenantRepo repo.TenantRepository,
    tenantUserRepo repo.TenantUserRepository,
) gin.HandlerFunc {
    return func(c *gin.Context) {
        newTenantID := c.GetHeader("X-Switch-Tenant")
        if newTenantID == "" {
            c.Next()
            return
        }
        
        userID := c.GetString("user_id")
        if userID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status": "error",
                "error":  "Authentication required for tenant switching",
            })
            c.Abort()
            return
        }
        
        // Validate user can access the new tenant
        tenantUser, err := tenantUserRepo.GetUserTenantAssociation(
            c.Request.Context(),
            userID,
            newTenantID,
        )
        if err != nil || tenantUser == nil || tenantUser.Status != models.TenantUserStatusActive {
            c.JSON(http.StatusForbidden, gin.H{
                "status": "error",
                "error":  "Cannot switch to requested tenant",
            })
            c.Abort()
            return
        }
        
        // Load the new tenant
        tenant, err := tenantRepo.GetTenant(c.Request.Context(), newTenantID)
        if err != nil || tenant.Status != models.TenantStatusActive {
            c.JSON(http.StatusForbidden, gin.H{
                "status": "error",
                "error":  "Tenant is not available",
            })
            c.Abort()
            return
        }
        
        // Get permissions for tenant role
        permissions := tenantUser.TenantRole.GetTenantPermissions()
        if len(tenantUser.AdditionalPermissions) > 0 {
            permissions = append(permissions, tenantUser.AdditionalPermissions...)
        }
        
        // Update context with new tenant
        c.Set("tenant_id", tenant.ID)
        c.Set("tenant", tenant)
        c.Set("tenant_user", tenantUser)
        c.Set("tenant_role", tenantUser.TenantRole)
        c.Set("tenant_permissions", permissions)
        
        c.Next()
    }
}

// RequireGlobalRole middleware ensures user has required global role
func RequireGlobalRole(requiredRole models.GlobalRole) gin.HandlerFunc {
    return func(c *gin.Context) {
        globalRole, exists := c.Get("global_role")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{
                "status": "error",
                "error":  "Authentication required",
            })
            c.Abort()
            return
        }
        
        userGlobalRole := globalRole.(models.GlobalRole)
        
        // Global Admin has access to everything
        if userGlobalRole == models.GlobalRoleAdmin {
            c.Next()
            return
        }
        
        // Check if user has required role
        if userGlobalRole != requiredRole {
            c.JSON(http.StatusForbidden, gin.H{
                "status": "error",
                "error":  fmt.Sprintf("Requires %s role", requiredRole),
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// RequireTenantRole middleware ensures user has required tenant role
func RequireTenantRole(requiredRole models.TenantRole) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Global Admin bypasses all tenant role checks
        isGlobalAdmin, _ := c.Get("is_global_admin")
        if isGlobalAdmin == true {
            c.Next()
            return
        }
        
        tenantRole, exists := c.Get("tenant_role")
        if !exists {
            c.JSON(http.StatusForbidden, gin.H{
                "status": "error",
                "error":  "No tenant access",
            })
            c.Abort()
            return
        }
        
        userTenantRole := tenantRole.(models.TenantRole)
        
        // Check role hierarchy: Admin > Editor > Guest
        roleLevel := map[models.TenantRole]int{
            models.TenantRoleGuest:  1,
            models.TenantRoleEditor: 2,
            models.TenantRoleAdmin:  3,
        }
        
        if roleLevel[userTenantRole] < roleLevel[requiredRole] {
            c.JSON(http.StatusForbidden, gin.H{
                "status": "error",
                "error":  fmt.Sprintf("Requires at least %s role in this tenant", requiredRole),
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// RequirePermission middleware checks for specific permission
func RequirePermission(permission string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Check global permissions first
        globalPerms, _ := c.Get("global_permissions")
        if globalPerms != nil {
            perms := globalPerms.([]string)
            for _, p := range perms {
                if p == permission || p == "platform.admin" {
                    c.Next()
                    return
                }
            }
        }
        
        // Check tenant permissions
        tenantPerms, _ := c.Get("tenant_permissions")
        if tenantPerms != nil {
            perms := tenantPerms.([]string)
            for _, p := range perms {
                if p == permission {
                    c.Next()
                    return
                }
            }
        }
        
        c.JSON(http.StatusForbidden, gin.H{
            "status": "error",
            "error":  fmt.Sprintf("Missing required permission: %s", permission),
        })
        c.Abort()
    }
}
```

### 4.2 Context Package

```go
// internal/utils/tenant_context.go
package utils

import (
    "context"
    "github.com/platformbuilds/mirador-core/internal/models"
)

type tenantContextKey string

const (
    TenantIDKey    tenantContextKey = "tenant_id"
    AccountIDKey   tenantContextKey = "account_id"
    TenantKey      tenantContextKey = "tenant"
)

// WithTenantID adds tenant ID to context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
    return context.WithValue(ctx, TenantIDKey, tenantID)
}

// GetTenantID retrieves tenant ID from context
func GetTenantID(ctx context.Context) string {
    if tenantID, ok := ctx.Value(TenantIDKey).(string); ok {
        return tenantID
    }
    return ""
}

// WithAccountID adds account ID to context
func WithAccountID(ctx context.Context, accountID string) context.Context {
    return context.WithValue(ctx, AccountIDKey, accountID)
}

// GetAccountID retrieves account ID from context
func GetAccountID(ctx context.Context) string {
    if accountID, ok := ctx.Value(AccountIDKey).(string); ok {
        return accountID
    }
    return ""
}

// WithTenant adds full tenant object to context
func WithTenant(ctx context.Context, tenant *models.Tenant) context.Context {
    return context.WithValue(ctx, TenantKey, tenant)
}

// GetTenant retrieves tenant from context
func GetTenant(ctx context.Context) *models.Tenant {
    if tenant, ok := ctx.Value(TenantKey).(*models.Tenant); ok {
        return tenant
    }
    return nil
}
```

---

## 5. Victoria* Services Integration

### 5.1 Intelligent Multi-Cluster Routing Layer

The core concept: **Mirador-Core dynamically routes requests to tenant-specific Victoria* clusters** based on:
- Tenant context (tenant_id)
- Cluster selection (cluster_id, environment, tags)
- Failover priority and health status

**Multi-Cluster Routing Architecture:**
```
Request → Tenant Context → Cluster Selection → Route to Specific Cluster
```

**Cluster Selection Strategy:**
1. **Explicit Cluster ID**: Use `cluster_id` from request if specified
2. **Environment Tag**: Use `environment` parameter (prod/staging/dev)
3. **Default Cluster**: Use tenant's default cluster for the data type
4. **Failover**: If selected cluster is unavailable, fallback to next priority cluster
5. **Tags-Based**: Select cluster matching specific tags

**Example Request with Cluster Selection:**
```json
{
  "query": "up{job='api'}",
  "cluster_id": "prod-us-east",
  "environment": "production"
}
```

### 5.2 Enhanced Request Models for Multi-Cluster Support

```go
// internal/models/queries.go - Enhanced models

// Add cluster selection fields to existing query models
type MetricsQLQueryRequest struct {
    Query    string `json:"query" binding:"required"`
    Time     string `json:"time,omitempty"`
    Timeout  string `json:"timeout,omitempty"`
    TenantID string `json:"-"` // Set by middleware
    
    // Multi-cluster support
    ClusterID   string   `json:"cluster_id,omitempty"`   // Explicit cluster ID
    Environment string   `json:"environment,omitempty"`  // Environment tag (prod/staging/dev)
    ClusterTags []string `json:"cluster_tags,omitempty"` // Tags for cluster selection
    
    // Optional: include definitions
    IncludeDefinitions *bool    `json:"include_definitions,omitempty"`
    DefinitionsMinimal *bool    `json:"definitions_minimal,omitempty"`
    LabelKeys          []string `json:"label_keys,omitempty"`
}

type LogsQLQueryRequest struct {
    Query    string `json:"query" binding:"required"`
    Start    string `json:"start,omitempty"`
    End      string `json:"end,omitempty"`
    Limit    int    `json:"limit,omitempty"`
    TenantID string `json:"-"`
    
    // Multi-cluster support
    ClusterID   string   `json:"cluster_id,omitempty"`
    Environment string   `json:"environment,omitempty"`
    ClusterTags []string `json:"cluster_tags,omitempty"`
}

type TraceSearchRequest struct {
    ServiceName string `json:"service_name,omitempty"`
    Operation   string `json:"operation,omitempty"`
    Start       string `json:"start" binding:"required"`
    End         string `json:"end" binding:"required"`
    Limit       int    `json:"limit,omitempty"`
    TenantID    string `json:"-"`
    
    // Multi-cluster support
    ClusterID   string   `json:"cluster_id,omitempty"`
    Environment string   `json:"environment,omitempty"`
    ClusterTags []string `json:"cluster_tags,omitempty"`
}
```

### 5.3 Enhanced VictoriaMetrics Service with Multi-Cluster Routing

```go
// internal/services/victoria_metrics_router.go
package services

import (
    "context"
    "fmt"
    "sync"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/internal/utils"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// VictoriaMetricsRouter handles routing to tenant-specific clusters
type VictoriaMetricsRouter struct {
    tenantRepo repo.TenantRepository
    logger     logger.Logger
    
    // Cache of cluster services (tenant_id:cluster_id -> service instance)
    clusterServices map[string]*VictoriaMetricsService
    mu              sync.RWMutex
}

func NewVictoriaMetricsRouter(
    tenantRepo repo.TenantRepository,
    logger logger.Logger,
) *VictoriaMetricsRouter {
    return &VictoriaMetricsRouter{
        tenantRepo:      tenantRepo,
        logger:          logger,
        clusterServices: make(map[string]*VictoriaMetricsService),
    }
}

// selectCluster chooses the appropriate cluster based on request parameters
func (r *VictoriaMetricsRouter) selectCluster(
    ctx context.Context,
    tenant *models.Tenant,
    clusterID string,
    environment string,
    tags []string,
) (*models.ClusterConfig, error) {
    clusters := tenant.Deployments.MetricsClusters
    
    // 1. Explicit cluster ID takes priority
    if clusterID != "" {
        for _, cluster := range clusters {
            if cluster.ClusterID == clusterID && cluster.Status == models.ClusterStatusActive {
                return &cluster, nil
            }
        }
        return nil, fmt.Errorf("cluster %s not found or inactive", clusterID)
    }
    
    // 2. Filter by environment if specified
    if environment != "" {
        for _, cluster := range clusters {
            if cluster.Environment == environment && cluster.Status == models.ClusterStatusActive {
                return &cluster, nil
            }
        }
        return nil, fmt.Errorf("no active cluster found for environment %s", environment)
    }
    
    // 3. Match by tags
    if len(tags) > 0 {
        for _, cluster := range clusters {
            if cluster.Status != models.ClusterStatusActive {
                continue
            }
            if matchesTags(cluster.Tags, tags) {
                return &cluster, nil
            }
        }
    }
    
    // 4. Use default cluster
    defaultClusterID := tenant.Deployments.DefaultMetricsCluster
    if defaultClusterID != "" {
        for _, cluster := range clusters {
            if cluster.ClusterID == defaultClusterID && cluster.Status == models.ClusterStatusActive {
                return &cluster, nil
            }
        }
    }
    
    // 5. Fallback to highest priority active cluster
    var selectedCluster *models.ClusterConfig
    highestPriority := -1
    for i, cluster := range clusters {
        if cluster.Status == models.ClusterStatusActive && cluster.Priority > highestPriority {
            selectedCluster = &clusters[i]
            highestPriority = cluster.Priority
        }
    }
    
    if selectedCluster == nil {
        return nil, fmt.Errorf("no active metrics cluster available for tenant")
    }
    
    return selectedCluster, nil
}

// matchesTags checks if cluster tags contain all required tags
func matchesTags(clusterTags, requiredTags []string) bool {
    tagSet := make(map[string]bool)
    for _, tag := range clusterTags {
        tagSet[tag] = true
    }
    
    for _, tag := range requiredTags {
        if !tagSet[tag] {
            return false
        }
    }
    return true
}

// GetServiceForCluster returns or creates a VictoriaMetrics service for the specific cluster
func (r *VictoriaMetricsRouter) GetServiceForCluster(
    ctx context.Context,
    tenantID string,
    clusterID string,
    environment string,
    tags []string,
) (*VictoriaMetricsService, *models.ClusterConfig, error) {
    // Load tenant
    tenant, err := r.tenantRepo.GetTenant(ctx, tenantID)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to load tenant: %w", err)
    }
    
    // Select appropriate cluster
    cluster, err := r.selectCluster(ctx, tenant, clusterID, environment, tags)
    if err != nil {
        return nil, nil, err
    }
    
    // Generate cache key
    cacheKey := fmt.Sprintf("%s:%s", tenantID, cluster.ClusterID)
    
    // Check cache first
    r.mu.RLock()
    if svc, exists := r.clusterServices[cacheKey]; exists {
        r.mu.RUnlock()
        return svc, cluster, nil
    }
    r.mu.RUnlock()
    
    // Create service for this cluster
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Double-check after acquiring write lock
    if svc, exists := r.clusterServices[cacheKey]; exists {
        return svc, cluster, nil
    }
    
    // Validate cluster has endpoints
    if len(cluster.Deployment.Endpoints) == 0 {
        return nil, nil, fmt.Errorf("no endpoints configured for cluster %s", cluster.ClusterID)
    }
    
    // Create service instance for this cluster
    svc := &VictoriaMetricsService{
        name:      fmt.Sprintf("metrics-%s-%s", tenant.Name, cluster.ClusterID),
        endpoints: cluster.Deployment.Endpoints,
        timeout:   time.Duration(cluster.Deployment.Timeout) * time.Millisecond,
        client: &http.Client{
            Timeout: time.Duration(cluster.Deployment.Timeout) * time.Millisecond,
        },
        logger:    r.logger,
        username:  cluster.Deployment.Username,
        password:  cluster.Deployment.Password,
        retries:   3,
        backoffMS: 1000,
    }
    
    // Cache the service
    r.clusterServices[cacheKey] = svc
    
    r.logger.Info("Created VictoriaMetrics service for cluster",
        "tenant_id", tenantID,
        "cluster_id", cluster.ClusterID,
        "environment", cluster.Environment,
        "endpoints", cluster.Deployment.Endpoints)
    
    return svc, cluster, nil
}

// InvalidateClusterCache removes cached service for specific cluster
func (r *VictoriaMetricsRouter) InvalidateClusterCache(tenantID, clusterID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    cacheKey := fmt.Sprintf("%s:%s", tenantID, clusterID)
    delete(r.clusterServices, cacheKey)
    r.logger.Info("Invalidated metrics cluster cache", 
        "tenant_id", tenantID, 
        "cluster_id", clusterID)
}

// InvalidateTenantCache removes all cached services for a tenant
func (r *VictoriaMetricsRouter) InvalidateTenantCache(tenantID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    prefix := tenantID + ":"
    for key := range r.clusterServices {
        if strings.HasPrefix(key, prefix) {
            delete(r.clusterServices, key)
        }
    }
    r.logger.Info("Invalidated all metrics clusters cache for tenant", "tenant_id", tenantID)
}
) *VictoriaMetricsRouter {
    return &VictoriaMetricsRouter{
        tenantRepo:     tenantRepo,
        logger:         logger,
        tenantServices: make(map[string]*VictoriaMetricsService),
    }
}

// ExecuteQueryWithCluster routes query to tenant-specific cluster
func (r *VictoriaMetricsRouter) ExecuteQueryWithCluster(
    ctx context.Context,
    request *models.MetricsQLQueryRequest,
) (*models.MetricsQLQueryResult, error) {
    tenantID := utils.GetTenantID(ctx)
    if tenantID == "" {
        return nil, fmt.Errorf("tenant context required")
    }
    
    // Get service for selected cluster
    svc, cluster, err := r.GetServiceForCluster(
        ctx, 
        tenantID, 
        request.ClusterID, 
        request.Environment,
        request.ClusterTags,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to get metrics service: %w", err)
    }
    
    r.logger.Debug("Executing query on cluster",
        "tenant_id", tenantID,
        "cluster_id", cluster.ClusterID,
        "environment", cluster.Environment,
        "query", request.Query)
    
    // Execute query on tenant's cluster
    result, err := svc.ExecuteQuery(ctx, request)
    if err != nil {
        return nil, err
    }
    
    // Add cluster metadata to result
    if result != nil {
        result.ClusterID = cluster.ClusterID
        result.Environment = cluster.Environment
    }
    
    return result, nil
}

// ExecuteRangeQueryWithCluster routes range query to tenant-specific cluster
func (r *VictoriaMetricsRouter) ExecuteRangeQueryWithCluster(
    ctx context.Context,
    request *models.MetricsQLRangeQueryRequest,
) (*models.MetricsQLRangeQueryResult, error) {
    tenantID := utils.GetTenantID(ctx)
    if tenantID == "" {
        return nil, fmt.Errorf("tenant context required")
    }
    
    svc, cluster, err := r.GetServiceForCluster(
        ctx,
        tenantID,
        request.ClusterID,
        request.Environment,
        request.ClusterTags,
    )
    if err != nil {
        return nil, err
    }
    
    result, err := svc.ExecuteRangeQuery(ctx, request)
    if err != nil {
        return nil, err
    }
    
    // Add cluster metadata
    if result != nil {
        result.ClusterID = cluster.ClusterID
        result.Environment = cluster.Environment
    }
    
    return result, nil
}

// GetSeriesWithCluster gets series from tenant-specific cluster
func (r *VictoriaMetricsRouter) GetSeriesWithCluster(
    ctx context.Context,
    request *models.SeriesRequest,
) ([]map[string]string, error) {
    tenantID := utils.GetTenantID(ctx)
    if tenantID == "" {
        return nil, fmt.Errorf("tenant context required")
    }
    
    svc, _, err := r.GetServiceForCluster(
        ctx,
        tenantID,
        request.ClusterID,
        request.Environment,
        request.ClusterTags,
    )
    if err != nil {
        return nil, err
    }
    
    return svc.GetSeries(ctx, request)
}

// ListClusters returns all available clusters for a tenant
func (r *VictoriaMetricsRouter) ListClusters(
    ctx context.Context,
    tenantID string,
    dataType string, // "metrics", "logs", "traces"
) ([]models.ClusterConfig, error) {
    tenant, err := r.tenantRepo.GetTenant(ctx, tenantID)
    if err != nil {
        return nil, err
    }
    
    switch dataType {
    case "metrics":
        return tenant.Deployments.MetricsClusters, nil
    case "logs":
        return tenant.Deployments.LogsClusters, nil
    case "traces":
        return tenant.Deployments.TracesClusters, nil
    default:
        return nil, fmt.Errorf("invalid data type: %s", dataType)
    }
}

// HealthCheckWithCluster checks health of specific cluster
func (r *VictoriaMetricsRouter) HealthCheckWithCluster(
    ctx context.Context,
    tenantID string,
    clusterID string,
) error {
    svc, _, err := r.GetServiceForCluster(ctx, tenantID, clusterID, "", nil)
    if err != nil {
        return err
    }
    
    return svc.HealthCheck(ctx)
}

// CreateTenantCluster initializes a new cluster for a tenant
func (r *VictoriaMetricsRouter) CreateTenantCluster(
    ctx context.Context,
    tenant *models.Tenant,
    cluster *models.ClusterConfig,
) error {
    // Validate cluster configuration
    if len(cluster.Deployment.Endpoints) == 0 {
        return fmt.Errorf("cluster must have at least one endpoint")
    }
    
    // Create cache key
    cacheKey := fmt.Sprintf("%s:%s", tenant.ID, cluster.ClusterID)
    
    // Create service for cluster
    svc := &VictoriaMetricsService{
        name:      fmt.Sprintf("metrics-%s-%s", tenant.Name, cluster.ClusterID),
        endpoints: cluster.Deployment.Endpoints,
        timeout:   time.Duration(cluster.Deployment.Timeout) * time.Millisecond,
        client: &http.Client{
            Timeout: time.Duration(cluster.Deployment.Timeout) * time.Millisecond,
        },
        logger:    r.logger,
        username:  cluster.Deployment.Username,
        password:  cluster.Deployment.Password,
        retries:   3,
        backoffMS: 1000,
    }
    
    // Verify connectivity
    if err := svc.HealthCheck(ctx); err != nil {
        return fmt.Errorf("cluster health check failed: %w", err)
    }
    
    // Cache the service
    r.mu.Lock()
    r.clusterServices[cacheKey] = svc
    r.mu.Unlock()
    
    r.logger.Info("Tenant metrics cluster initialized",
        "tenant_id", tenant.ID,
        "cluster_id", cluster.ClusterID,
        "environment", cluster.Environment,
        "endpoints", cluster.Deployment.Endpoints)
    
    return nil
}

// DeleteTenantCluster removes cluster from cache
func (r *VictoriaMetricsRouter) DeleteTenantCluster(
    ctx context.Context,
    tenantID string,
    clusterID string,
) error {
    r.InvalidateClusterCache(tenantID, clusterID)
    
    r.logger.Info("Tenant metrics cluster removed from cache",
        "tenant_id", tenantID,
        "cluster_id", clusterID,
        "note", "physical deployment cleanup handled externally")
    
    return nil
}
```

### 5.3 Enhanced VictoriaLogs Service with Routing

```go
// internal/services/victoria_logs_router.go
package services

import (
    "context"
    "fmt"
    "sync"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/internal/utils"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// VictoriaLogsRouter routes to tenant-specific log deployments
type VictoriaLogsRouter struct {
    tenantRepo     repo.TenantRepository
    logger         logger.Logger
    tenantServices map[string]*VictoriaLogsService
    mu             sync.RWMutex
}

func NewVictoriaLogsRouter(
    tenantRepo repo.TenantRepository,
    logger logger.Logger,
) *VictoriaLogsRouter {
    return &VictoriaLogsRouter{
        tenantRepo:     tenantRepo,
        logger:         logger,
        tenantServices: make(map[string]*VictoriaLogsService),
    }
}

// GetServiceForTenant returns logs service for tenant
func (r *VictoriaLogsRouter) GetServiceForTenant(
    ctx context.Context,
    tenantID string,
) (*VictoriaLogsService, error) {
    r.mu.RLock()
    if svc, exists := r.tenantServices[tenantID]; exists {
        r.mu.RUnlock()
        return svc, nil
    }
    r.mu.RUnlock()
    
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if svc, exists := r.tenantServices[tenantID]; exists {
        return svc, nil
    }
    
    tenant, err := r.tenantRepo.GetTenant(ctx, tenantID)
    if err != nil {
        return nil, fmt.Errorf("failed to load tenant: %w", err)
    }
    
    if len(tenant.Deployments.Logs.Endpoints) == 0 {
        return nil, fmt.Errorf("no logs endpoints configured for tenant %s", tenantID)
    }
    
    svc := &VictoriaLogsService{
        name:      fmt.Sprintf("logs-%s", tenant.Name),
        endpoints: tenant.Deployments.Logs.Endpoints,
        timeout:   time.Duration(tenant.Deployments.Logs.Timeout) * time.Millisecond,
        client: &http.Client{
            Timeout: time.Duration(tenant.Deployments.Logs.Timeout) * time.Millisecond,
        },
        logger:    r.logger,
        username:  tenant.Deployments.Logs.Username,
        password:  tenant.Deployments.Logs.Password,
        retries:   3,
        backoffMS: 1000,
    }
    
    r.tenantServices[tenantID] = svc
    
    r.logger.Info("Created VictoriaLogs service for tenant",
        "tenant_id", tenantID,
        "endpoints", tenant.Deployments.Logs.Endpoints)
    
    return svc, nil
}

// ExecuteQueryWithTenant routes log query to tenant deployment
func (r *VictoriaLogsRouter) ExecuteQueryWithTenant(
    ctx context.Context,
    request *models.LogsQLQueryRequest,
) (*models.LogsQLQueryResult, error) {
    tenantID := utils.GetTenantID(ctx)
    if tenantID == "" {
        return nil, fmt.Errorf("tenant context required")
    }
    
    svc, err := r.GetServiceForTenant(ctx, tenantID)
    if err != nil {
        return nil, err
    }
    
    return svc.ExecuteQuery(ctx, request)
}

// StoreJSONEventWithTenant stores event to tenant's log deployment
func (r *VictoriaLogsRouter) StoreJSONEventWithTenant(
    ctx context.Context,
    event map[string]interface{},
) error {
    tenantID := utils.GetTenantID(ctx)
    if tenantID == "" {
        return fmt.Errorf("tenant context required")
    }
    
    svc, err := r.GetServiceForTenant(ctx, tenantID)
    if err != nil {
        return err
    }
    
    // Add tenant metadata to event
    event["_tenant_id"] = tenantID
    
    return svc.StoreJSONEvent(ctx, event, "") // No AccountID needed
}

// InvalidateTenantCache removes cached service
func (r *VictoriaLogsRouter) InvalidateTenantCache(tenantID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.tenantServices, tenantID)
    r.logger.Info("Invalidated logs service cache", "tenant_id", tenantID)
}
```

### 5.4 Enhanced VictoriaTraces Service with Routing

```go
// internal/services/victoria_traces_router.go
package services

import (
    "context"
    "fmt"
    "sync"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/internal/utils"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// VictoriaTracesRouter routes to tenant-specific trace deployments
type VictoriaTracesRouter struct {
    tenantRepo     repo.TenantRepository
    logger         logger.Logger
    tenantServices map[string]*VictoriaTracesService
    mu             sync.RWMutex
}

func NewVictoriaTracesRouter(
    tenantRepo repo.TenantRepository,
    logger logger.Logger,
) *VictoriaTracesRouter {
    return &VictoriaTracesRouter{
        tenantRepo:     tenantRepo,
        logger:         logger,
        tenantServices: make(map[string]*VictoriaTracesService),
    }
}

// GetServiceForTenant returns traces service for tenant
func (r *VictoriaTracesRouter) GetServiceForTenant(
    ctx context.Context,
    tenantID string,
) (*VictoriaTracesService, error) {
    r.mu.RLock()
    if svc, exists := r.tenantServices[tenantID]; exists {
        r.mu.RUnlock()
        return svc, nil
    }
    r.mu.RUnlock()
    
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if svc, exists := r.tenantServices[tenantID]; exists {
        return svc, nil
    }
    
    tenant, err := r.tenantRepo.GetTenant(ctx, tenantID)
    if err != nil {
        return nil, fmt.Errorf("failed to load tenant: %w", err)
    }
    
    if len(tenant.Deployments.Traces.Endpoints) == 0 {
        return nil, fmt.Errorf("no traces endpoints configured for tenant %s", tenantID)
    }
    
    svc := &VictoriaTracesService{
        name:      fmt.Sprintf("traces-%s", tenant.Name),
        endpoints: tenant.Deployments.Traces.Endpoints,
        timeout:   time.Duration(tenant.Deployments.Traces.Timeout) * time.Millisecond,
        client: &http.Client{
            Timeout: time.Duration(tenant.Deployments.Traces.Timeout) * time.Millisecond,
        },
        logger:   r.logger,
        username: tenant.Deployments.Traces.Username,
        password: tenant.Deployments.Traces.Password,
    }
    
    r.tenantServices[tenantID] = svc
    
    r.logger.Info("Created VictoriaTraces service for tenant",
        "tenant_id", tenantID,
        "endpoints", tenant.Deployments.Traces.Endpoints)
    
    return svc, nil
}

// SearchTracesWithTenant routes trace search to tenant deployment
func (r *VictoriaTracesRouter) SearchTracesWithTenant(
    ctx context.Context,
    request *models.TraceSearchRequest,
) (*models.TraceSearchResult, error) {
    tenantID := utils.GetTenantID(ctx)
    if tenantID == "" {
        return nil, fmt.Errorf("tenant context required")
    }
    
    svc, err := r.GetServiceForTenant(ctx, tenantID)
    if err != nil {
        return nil, err
    }
    
    return svc.SearchTraces(ctx, request)
}

// GetOperationsWithTenant gets operations from tenant deployment
func (r *VictoriaTracesRouter) GetOperationsWithTenant(
    ctx context.Context,
    serviceName string,
) ([]string, error) {
    tenantID := utils.GetTenantID(ctx)
    if tenantID == "" {
        return nil, fmt.Errorf("tenant context required")
    }
    
    svc, err := r.GetServiceForTenant(ctx, tenantID)
    if err != nil {
        return nil, err
    }
    
    return svc.GetOperations(ctx, serviceName, "")
}

// InvalidateTenantCache removes cached service
func (r *VictoriaTracesRouter) InvalidateTenantCache(tenantID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.tenantServices, tenantID)
    r.logger.Info("Invalidated traces service cache", "tenant_id", tenantID)
}
```

### 5.5 Multi-Cluster Usage Examples

#### Example 1: Tenant Configuration with Multiple Clusters

```json
{
  "id": "tenant-acme-corp",
  "name": "acme-corp",
  "display_name": "ACME Corporation",
  "deployments": {
    "metrics_clusters": [
      {
        "cluster_id": "prod-us-east",
        "name": "Production US East",
        "description": "Primary production metrics cluster",
        "environment": "production",
        "deployment": {
          "endpoints": [
            "https://vm-prod-us-east-1.acme.internal:8428",
            "https://vm-prod-us-east-2.acme.internal:8428"
          ],
          "timeout": 30000,
          "max_connections": 200,
          "namespace": "acme-prod-us-east",
          "cluster": "prod-us-east-k8s",
          "region": "us-east-1"
        },
        "status": "active",
        "priority": 100,
        "tags": ["production", "primary", "us-east"]
      },
      {
        "cluster_id": "prod-eu-west",
        "name": "Production EU West",
        "description": "EU production metrics cluster (GDPR compliant)",
        "environment": "production",
        "deployment": {
          "endpoints": [
            "https://vm-prod-eu-west-1.acme.internal:8428"
          ],
          "timeout": 30000,
          "namespace": "acme-prod-eu-west",
          "region": "eu-west-1"
        },
        "status": "active",
        "priority": 90,
        "tags": ["production", "eu", "gdpr"]
      },
      {
        "cluster_id": "staging",
        "name": "Staging Cluster",
        "environment": "staging",
        "deployment": {
          "endpoints": ["https://vm-staging.acme.internal:8428"],
          "timeout": 20000
        },
        "status": "active",
        "priority": 50,
        "tags": ["staging", "non-production"]
      },
      {
        "cluster_id": "dev",
        "name": "Development Cluster",
        "environment": "development",
        "deployment": {
          "endpoints": ["https://vm-dev.acme.internal:8428"],
          "timeout": 15000
        },
        "status": "active",
        "priority": 10,
        "tags": ["dev", "testing"]
      }
    ],
    "logs_clusters": [
      {
        "cluster_id": "prod-logs",
        "name": "Production Logs",
        "environment": "production",
        "deployment": {
          "endpoints": ["https://vl-prod.acme.internal:9428"]
        },
        "status": "active",
        "priority": 100
      },
      {
        "cluster_id": "staging-logs",
        "name": "Staging Logs",
        "environment": "staging",
        "deployment": {
          "endpoints": ["https://vl-staging.acme.internal:9428"]
        },
        "status": "active",
        "priority": 50
      }
    ],
    "traces_clusters": [
      {
        "cluster_id": "prod-traces",
        "name": "Production Traces",
        "environment": "production",
        "deployment": {
          "endpoints": ["https://vt-prod.acme.internal:4318"]
        },
        "status": "active",
        "priority": 100
      }
    ],
    "default_metrics_cluster": "prod-us-east",
    "default_logs_cluster": "prod-logs",
    "default_traces_cluster": "prod-traces"
  }
}
```

#### Example 2: API Requests Targeting Different Clusters

**Query Production US Cluster Explicitly:**
```bash
POST /api/v1/query/metrics
{
  "query": "sum(rate(http_requests_total[5m])) by (service)",
  "cluster_id": "prod-us-east"
}
```

**Query EU Cluster for GDPR Compliance:**
```bash
POST /api/v1/query/metrics
{
  "query": "avg(response_time_seconds) by (endpoint)",
  "cluster_id": "prod-eu-west"
}
```

**Query Staging Environment:**
```bash
POST /api/v1/query/metrics
{
  "query": "up{job='api'}",
  "environment": "staging"
}
```

**Query Using Tags:**
```bash
POST /api/v1/query/metrics
{
  "query": "node_cpu_usage",
  "cluster_tags": ["production", "us-east"]
}
```

**Query Logs from Staging:**
```bash
POST /api/v1/query/logs
{
  "query": "error | logfmt",
  "cluster_id": "staging-logs",
  "start": "2024-11-08T00:00:00Z",
  "end": "2024-11-08T23:59:59Z"
}
```

**Search Traces in Production:**
```bash
POST /api/v1/query/traces
{
  "service_name": "order-service",
  "operation": "process_order",
  "cluster_id": "prod-traces",
  "start": "2024-11-08T00:00:00Z",
  "end": "2024-11-08T23:59:59Z"
}
```

**Default Cluster (No Cluster Specified):**
```bash
POST /api/v1/query/metrics
{
  "query": "up"
}
# Routes to default_metrics_cluster: "prod-us-east"
```

#### Example 3: Multi-Cluster Correlation Queries

**Compare Production Clusters:**
```bash
POST /api/v1/correlation/analyze
{
  "metrics_query": "sum(rate(errors_total[5m]))",
  "logs_query": "error",
  "start": "2024-11-08T10:00:00Z",
  "end": "2024-11-08T11:00:00Z",
  "clusters": [
    {
      "cluster_id": "prod-us-east",
      "types": ["metrics", "logs"]
    },
    {
      "cluster_id": "prod-eu-west",
      "types": ["metrics", "logs"]
    }
  ]
}
```

#### Example 4: Cluster Management API

**List Available Clusters:**
```bash
GET /api/v1/tenants/{tenant_id}/clusters?type=metrics

Response:
{
  "status": "success",
  "data": {
    "clusters": [
      {
        "cluster_id": "prod-us-east",
        "name": "Production US East",
        "environment": "production",
        "status": "active",
        "priority": 100,
        "endpoints": ["https://vm-prod-us-east-1.acme.internal:8428"],
        "health": "healthy"
      },
      {
        "cluster_id": "staging",
        "name": "Staging Cluster",
        "environment": "staging",
        "status": "active",
        "priority": 50,
        "health": "healthy"
      }
    ]
  }
}
```

**Add New Cluster:**
```bash
POST /api/v1/tenants/{tenant_id}/clusters
{
  "data_type": "metrics",
  "cluster": {
    "cluster_id": "prod-ap-south",
    "name": "Production APAC",
    "environment": "production",
    "deployment": {
      "endpoints": ["https://vm-prod-ap.acme.internal:8428"],
      "timeout": 30000
    },
    "priority": 95,
    "tags": ["production", "apac"]
  }
}
```

**Update Cluster Status:**
```bash
PATCH /api/v1/tenants/{tenant_id}/clusters/staging
{
  "status": "maintenance"
}
```

**Health Check Specific Cluster:**
```bash
GET /api/v1/tenants/{tenant_id}/clusters/prod-us-east/health

Response:
{
  "status": "success",
  "data": {
    "cluster_id": "prod-us-east",
    "status": "active",
    "health": "healthy",
    "endpoints": [
      {
        "url": "https://vm-prod-us-east-1.acme.internal:8428",
        "status": "healthy",
        "response_time_ms": 45
      },
      {
        "url": "https://vm-prod-us-east-2.acme.internal:8428",
        "status": "healthy",
        "response_time_ms": 52
      }
    ]
  }
}
```

#### Example 5: Failover Scenario

```go
// When primary cluster fails, router automatically selects next priority cluster

// User queries prod-us-east which is down
request := &models.MetricsQLQueryRequest{
    Query:     "up",
    ClusterID: "prod-us-east", // This cluster is down
}

// Router detects cluster is unhealthy and fails over:
// 1. Checks prod-us-east (priority 100) - FAILED
// 2. Falls back to prod-eu-west (priority 90) - SUCCESS
// 3. Returns data with metadata indicating failover occurred

response := &models.MetricsQLQueryResult{
    Status: "success",
    Data:   [...],
    ClusterID: "prod-eu-west",
    Environment: "production",
    Metadata: {
        "requested_cluster": "prod-us-east",
        "actual_cluster": "prod-eu-west",
        "failover": true,
        "failover_reason": "requested cluster unhealthy"
    }
}
```

---

## 6. Repository Layer Enhancement

### 6.1 Tenant Repository Interface

```go
// internal/repo/tenant_repo.go
package repo

import (
    "context"
    "github.com/platformbuilds/mirador-core/internal/models"
)

// UserRepository manages global user entities
type UserRepository interface {
    // CRUD operations
    CreateUser(ctx context.Context, user *models.User) error
    GetUser(ctx context.Context, userID string) (*models.User, error)
    GetUserByEmail(ctx context.Context, email string) (*models.User, error)
    GetUserByUsername(ctx context.Context, username string) (*models.User, error)
    UpdateUser(ctx context.Context, user *models.User) error
    DeleteUser(ctx context.Context, userID string) error
    ListUsers(ctx context.Context, limit, offset int) ([]*models.User, int, error)
    
    // Status management
    ActivateUser(ctx context.Context, userID string) error
    SuspendUser(ctx context.Context, userID string) error
    
    // Authentication
    UpdatePassword(ctx context.Context, userID, passwordHash string) error
    VerifyEmail(ctx context.Context, userID string) error
}

// TenantRepository manages tenant entities
type TenantRepository interface {
    // CRUD operations
    CreateTenant(ctx context.Context, tenant *models.Tenant) error
    GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error)
    GetTenantByAccountID(ctx context.Context, accountID string) (*models.Tenant, error)
    UpdateTenant(ctx context.Context, tenant *models.Tenant) error
    DeleteTenant(ctx context.Context, tenantID string) error
    ListTenants(ctx context.Context, limit, offset int) ([]*models.Tenant, int, error)
    
    // Status management
    SuspendTenant(ctx context.Context, tenantID string) error
    ActivateTenant(ctx context.Context, tenantID string) error
    
    // Configuration
    GetTenantConfig(ctx context.Context, tenantID string) (*models.TenantConfig, error)
    UpdateTenantConfig(ctx context.Context, config *models.TenantConfig) error
    
    // Quota management
    UpdateTenantQuotas(ctx context.Context, tenantID string, quotas models.TenantQuotas) error
    CheckQuotaLimit(ctx context.Context, tenantID string, quotaType string) (bool, error)
}

// TenantUserRepository manages user-tenant associations
type TenantUserRepository interface {
    // User-tenant associations
    AddUserToTenant(ctx context.Context, tenantUser *models.TenantUser) error
    RemoveUserFromTenant(ctx context.Context, userID, tenantID string) error
    UpdateUserRoles(ctx context.Context, userID, tenantID string, roles []string) error
    
    // Query associations
    GetUserTenants(ctx context.Context, userID string) ([]*models.TenantUser, error)
    GetTenantUsers(ctx context.Context, tenantID string) ([]*models.TenantUser, error)
    GetUserTenantAssociation(ctx context.Context, userID, tenantID string) (*models.TenantUser, error)
    
    // Validation
    ValidateUserAccess(ctx context.Context, userID, tenantID string) (bool, error)
    GetUserRolesInTenant(ctx context.Context, userID, tenantID string) ([]string, error)
    GetUserPermissionsInTenant(ctx context.Context, userID, tenantID string) ([]string, error)
    
    // Bulk operations
    GetUsersInMultipleTenants(ctx context.Context, userID string, tenantIDs []string) ([]*models.TenantUser, error)
}

// RoleRepository manages tenant-specific roles
type RoleRepository interface {
    // CRUD operations
    CreateRole(ctx context.Context, role *models.Role) error
    GetRole(ctx context.Context, roleID string) (*models.Role, error)
    GetRoleByName(ctx context.Context, tenantID, roleName string) (*models.Role, error)
    UpdateRole(ctx context.Context, role *models.Role) error
    DeleteRole(ctx context.Context, roleID string) error
    ListRoles(ctx context.Context, tenantID string) ([]*models.Role, error)
    
    // Permission management
    GetRolePermissions(ctx context.Context, roleID string) ([]string, error)
    UpdateRolePermissions(ctx context.Context, roleID string, permissions []string) error
}
```

### 6.2 Weaviate-based Implementation

```go
// internal/repo/tenant_weaviate.go
package repo

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/storage/weaviate"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

const TenantClass = "Tenant"

type WeaviateTenantRepo struct {
    client *weaviate.Client
    logger logger.Logger
}

func NewWeaviateTenantRepo(client *weaviate.Client, log logger.Logger) *WeaviateTenantRepo {
    return &WeaviateTenantRepo{
        client: client,
        logger: log,
    }
}

// CreateTenant creates a new tenant in Weaviate
func (r *WeaviateTenantRepo) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
    if tenant.ID == "" {
        tenant.ID = generateTenantID()
    }
    
    tenant.CreatedAt = time.Now()
    tenant.UpdatedAt = time.Now()
    tenant.Status = models.TenantStatusActive
    
    properties := map[string]interface{}{
        "tenant_id":    tenant.ID,
        "name":         tenant.Name,
        "display_name": tenant.DisplayName,
        "description":  tenant.Description,
        "account_id":   tenant.AccountID,
        "status":       string(tenant.Status),
        "admin_email":  tenant.AdminEmail,
        "admin_name":   tenant.AdminName,
        "created_at":   tenant.CreatedAt.Format(time.RFC3339),
        "updated_at":   tenant.UpdatedAt.Format(time.RFC3339),
    }
    
    // Add quotas
    quotasJSON, _ := json.Marshal(tenant.Quotas)
    properties["quotas"] = string(quotasJSON)
    
    // Add features
    featuresJSON, _ := json.Marshal(tenant.Features)
    properties["features"] = string(featuresJSON)
    
    err := r.client.CreateObject(ctx, TenantClass, properties)
    if err != nil {
        return fmt.Errorf("failed to create tenant: %w", err)
    }
    
    r.logger.Info("Tenant created", "tenant_id", tenant.ID, "account_id", tenant.AccountID)
    return nil
}

// GetTenant retrieves tenant by ID
func (r *WeaviateTenantRepo) GetTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
    query := fmt.Sprintf(`{tenant_id: "%s"}`, tenantID)
    
    objects, err := r.client.QueryObjects(ctx, TenantClass, query, 1)
    if err != nil {
        return nil, err
    }
    
    if len(objects) == 0 {
        return nil, fmt.Errorf("tenant not found: %s", tenantID)
    }
    
    return r.objectToTenant(objects[0])
}

// UpdateTenant updates tenant information
func (r *WeaviateTenantRepo) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
    tenant.UpdatedAt = time.Now()
    
    properties := map[string]interface{}{
        "name":         tenant.Name,
        "display_name": tenant.DisplayName,
        "description":  tenant.Description,
        "status":       string(tenant.Status),
        "admin_email":  tenant.AdminEmail,
        "admin_name":   tenant.AdminName,
        "updated_at":   tenant.UpdatedAt.Format(time.RFC3339),
    }
    
    // Update quotas if changed
    quotasJSON, _ := json.Marshal(tenant.Quotas)
    properties["quotas"] = string(quotasJSON)
    
    // Update features if changed
    featuresJSON, _ := json.Marshal(tenant.Features)
    properties["features"] = string(featuresJSON)
    
    // TODO: Implement Weaviate update logic
    return nil
}

// ValidateUserAccess checks if user can access tenant
func (r *WeaviateTenantRepo) ValidateUserAccess(
    ctx context.Context,
    userID, tenantID string,
) (bool, error) {
    // Query TenantUser association
    query := fmt.Sprintf(`{tenant_id: "%s", user_id: "%s"}`, tenantID, userID)
    
    objects, err := r.client.QueryObjects(ctx, "TenantUser", query, 1)
    if err != nil {
        return false, err
    }
    
    return len(objects) > 0, nil
}

func (r *WeaviateTenantRepo) objectToTenant(obj map[string]interface{}) (*models.Tenant, error) {
    tenant := &models.Tenant{}
    
    if id, ok := obj["tenant_id"].(string); ok {
        tenant.ID = id
    }
    if name, ok := obj["name"].(string); ok {
        tenant.Name = name
    }
    if accountID, ok := obj["account_id"].(string); ok {
        tenant.AccountID = accountID
    }
    if status, ok := obj["status"].(string); ok {
        tenant.Status = models.TenantStatus(status)
    }
    
    // Parse quotas
    if quotasStr, ok := obj["quotas"].(string); ok {
        json.Unmarshal([]byte(quotasStr), &tenant.Quotas)
    }
    
    // Parse features
    if featuresStr, ok := obj["features"].(string); ok {
        json.Unmarshal([]byte(featuresStr), &tenant.Features)
    }
    
    return tenant, nil
}

func generateTenantID() string {
    return fmt.Sprintf("tenant_%d", time.Now().UnixNano())
}
```

---

## 7. API Layer Updates

### 7.1 User Management Handlers

```go
// internal/api/handlers/user.handler.go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

type UserHandler struct {
    userRepo       repo.UserRepository
    tenantUserRepo repo.TenantUserRepository
    logger         logger.Logger
}

func NewUserHandler(
    userRepo repo.UserRepository,
    tenantUserRepo repo.TenantUserRepository,
    log logger.Logger,
) *UserHandler {
    return &UserHandler{
        userRepo:       userRepo,
        tenantUserRepo: tenantUserRepo,
        logger:         log,
    }
}

// CreateUser creates a new global user
func (h *UserHandler) CreateUser(c *gin.Context) {
    var req models.User
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "status": "error",
            "error":  "Invalid request body",
            "detail": err.Error(),
        })
        return
    }
    
    // Check if user already exists
    existing, _ := h.userRepo.GetUserByEmail(c.Request.Context(), req.Email)
    if existing != nil {
        c.JSON(http.StatusConflict, gin.H{
            "status": "error",
            "error":  "User with this email already exists",
        })
        return
    }
    
    // Create user
    if err := h.userRepo.CreateUser(c.Request.Context(), &req); err != nil {
        h.logger.Error("Failed to create user", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to create user",
        })
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{
        "status": "success",
        "user":   req,
    })
}

// GetUser retrieves a user by ID
func (h *UserHandler) GetUser(c *gin.Context) {
    userID := c.Param("id")
    
    user, err := h.userRepo.GetUser(c.Request.Context(), userID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "status": "error",
            "error":  "User not found",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status": "success",
        "user":   user,
    })
}

// GetUserTenants retrieves all tenants a user has access to
func (h *UserHandler) GetUserTenants(c *gin.Context) {
    userID := c.Param("id")
    
    tenantUsers, err := h.tenantUserRepo.GetUserTenants(c.Request.Context(), userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to retrieve user tenants",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status":  "success",
        "tenants": tenantUsers,
    })
}
```

### 7.2 Tenant User Management Handlers

```go
// internal/api/handlers/tenant.handler.go
package handlers

// AddUserToTenant adds a user to a tenant with specific roles
func (h *TenantHandler) AddUserToTenant(c *gin.Context) {
    tenantID := c.Param("id")
    
    var req struct {
        UserID      string   `json:"user_id" binding:"required"`
        Roles       []string `json:"roles" binding:"required,min=1"`
        Permissions []string `json:"permissions,omitempty"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "status": "error",
            "error":  "Invalid request body",
        })
        return
    }
    
    // Verify user exists
    user, err := h.userRepo.GetUser(c.Request.Context(), req.UserID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "status": "error",
            "error":  "User not found",
        })
        return
    }
    
    // Verify tenant exists
    tenant, err := h.tenantRepo.GetTenant(c.Request.Context(), tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "status": "error",
            "error":  "Tenant not found",
        })
        return
    }
    
    // Check if user already has access
    existing, _ := h.tenantUserRepo.GetUserTenantAssociation(
        c.Request.Context(),
        req.UserID,
        tenantID,
    )
    if existing != nil {
        c.JSON(http.StatusConflict, gin.H{
            "status": "error",
            "error":  "User already has access to this tenant",
        })
        return
    }
    
    // Create tenant user association
    inviterID := c.GetString("user_id")
    tenantUser := &models.TenantUser{
        TenantID:    tenantID,
        UserID:      req.UserID,
        Roles:       req.Roles,
        Permissions: req.Permissions,
        Status:      models.TenantUserStatusActive,
        JoinedAt:    time.Now(),
        InvitedBy:   inviterID,
        InvitedAt:   time.Now(),
    }
    
    if err := h.tenantUserRepo.AddUserToTenant(c.Request.Context(), tenantUser); err != nil {
        h.logger.Error("Failed to add user to tenant", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to add user to tenant",
        })
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{
        "status":      "success",
        "tenant_user": tenantUser,
    })
}

// UpdateUserRoles updates a user's roles in a tenant
func (h *TenantHandler) UpdateUserRoles(c *gin.Context) {
    tenantID := c.Param("id")
    userID := c.Param("userId")
    
    var req struct {
        Roles       []string `json:"roles" binding:"required,min=1"`
        Permissions []string `json:"permissions,omitempty"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "status": "error",
            "error":  "Invalid request body",
        })
        return
    }
    
    // Update roles
    if err := h.tenantUserRepo.UpdateUserRoles(c.Request.Context(), userID, tenantID, req.Roles); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to update user roles",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status":  "success",
        "message": "User roles updated successfully",
    })
}

// RemoveUserFromTenant removes a user's access to a tenant
func (h *TenantHandler) RemoveUserFromTenant(c *gin.Context) {
    tenantID := c.Param("id")
    userID := c.Param("userId")
    
    if err := h.tenantUserRepo.RemoveUserFromTenant(c.Request.Context(), userID, tenantID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to remove user from tenant",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status":  "success",
        "message": "User removed from tenant successfully",
    })
}

// ListTenantUsers lists all users with access to a tenant
func (h *TenantHandler) ListTenantUsers(c *gin.Context) {
    tenantID := c.Param("id")
    
    tenantUsers, err := h.tenantUserRepo.GetTenantUsers(c.Request.Context(), tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to list tenant users",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status": "success",
        "users":  tenantUsers,
        "total":  len(tenantUsers),
    })
}
```

### 7.3 Tenant Management Handlers

```go
// internal/api/handlers/tenant.handler.go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/internal/services"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

type TenantHandler struct {
    tenantRepo     repo.TenantRepository
    metricsService *services.VictoriaMetricsService
    logsService    *services.VictoriaLogsService
    tracesService  *services.VictoriaTracesService
    logger         logger.Logger
}

func NewTenantHandler(
    tenantRepo repo.TenantRepository,
    metrics *services.VictoriaMetricsService,
    logs *services.VictoriaLogsService,
    traces *services.VictoriaTracesService,
    log logger.Logger,
) *TenantHandler {
    return &TenantHandler{
        tenantRepo:     tenantRepo,
        metricsService: metrics,
        logsService:    logs,
        tracesService:  traces,
        logger:         log,
    }
}

// CreateTenant creates a new tenant
func (h *TenantHandler) CreateTenant(c *gin.Context) {
    var req models.Tenant
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "status": "error",
            "error":  "Invalid request body",
            "detail": err.Error(),
        })
        return
    }
    
    // Validate account ID is numeric
    if !isNumericAccountID(req.AccountID) {
        c.JSON(http.StatusBadRequest, gin.H{
            "status": "error",
            "error":  "Account ID must be numeric for Victoria* services",
        })
        return
    }
    
    // Create tenant in database
    if err := h.tenantRepo.CreateTenant(c.Request.Context(), &req); err != nil {
        h.logger.Error("Failed to create tenant", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to create tenant",
        })
        return
    }
    
    // Initialize Victoria* services for tenant
    ctx := c.Request.Context()
    
    // Initialize metrics
    if err := h.metricsService.CreateTenantMetrics(ctx, &req); err != nil {
        h.logger.Warn("Failed to init tenant metrics", "error", err)
    }
    
    // Initialize logs
    if err := h.logsService.CreateTenantLogs(ctx, &req); err != nil {
        h.logger.Warn("Failed to init tenant logs", "error", err)
    }
    
    // Initialize traces
    if err := h.tracesService.CreateTenantTraces(ctx, &req); err != nil {
        h.logger.Warn("Failed to init tenant traces", "error", err)
    }
    
    c.JSON(http.StatusCreated, gin.H{
        "status": "success",
        "tenant": req,
    })
}

// GetTenant retrieves tenant by ID
func (h *TenantHandler) GetTenant(c *gin.Context) {
    tenantID := c.Param("id")
    
    tenant, err := h.tenantRepo.GetTenant(c.Request.Context(), tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "status": "error",
            "error":  "Tenant not found",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status": "success",
        "tenant": tenant,
    })
}

// UpdateTenant updates tenant information
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
    tenantID := c.Param("id")
    
    var updates models.Tenant
    if err := c.ShouldBindJSON(&updates); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "status": "error",
            "error":  "Invalid request body",
        })
        return
    }
    
    // Get existing tenant
    tenant, err := h.tenantRepo.GetTenant(c.Request.Context(), tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "status": "error",
            "error":  "Tenant not found",
        })
        return
    }
    
    // Apply updates
    tenant.Name = updates.Name
    tenant.DisplayName = updates.DisplayName
    tenant.Description = updates.Description
    tenant.AdminEmail = updates.AdminEmail
    tenant.AdminName = updates.AdminName
    
    if err := h.tenantRepo.UpdateTenant(c.Request.Context(), tenant); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to update tenant",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status": "success",
        "tenant": tenant,
    })
}

// DeleteTenant soft-deletes a tenant
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
    tenantID := c.Param("id")
    
    tenant, err := h.tenantRepo.GetTenant(c.Request.Context(), tenantID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "status": "error",
            "error":  "Tenant not found",
        })
        return
    }
    
    // Soft delete - mark as deleted
    tenant.Status = models.TenantStatusDeleted
    
    if err := h.tenantRepo.UpdateTenant(c.Request.Context(), tenant); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to delete tenant",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status": "success",
        "message": "Tenant deleted successfully",
    })
}

// ListTenants retrieves all tenants
func (h *TenantHandler) ListTenants(c *gin.Context) {
    limit := 50
    offset := 0
    
    // Parse query params
    if l := c.Query("limit"); l != "" {
        fmt.Sscanf(l, "%d", &limit)
    }
    if o := c.Query("offset"); o != "" {
        fmt.Sscanf(o, "%d", &offset)
    }
    
    tenants, total, err := h.tenantRepo.ListTenants(c.Request.Context(), limit, offset)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status": "error",
            "error":  "Failed to list tenants",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status":  "success",
        "tenants": tenants,
        "total":   total,
        "limit":   limit,
        "offset":  offset,
    })
}

func isNumericAccountID(id string) bool {
    // Simple validation - can be enhanced
    if id == "" {
        return false
    }
    for _, ch := range id {
        if ch < '0' || ch > '9' {
            return false
        }
    }
    return true
}
```

### 7.4 API Routes

```go
// internal/api/server.go - Add user and tenant routes
func (s *Server) setupRoutes() {
    // ... existing routes ...
    
    v1 := s.router.Group("/api/v1")
    
    // Global User management routes
    userHandler := handlers.NewUserHandler(
        s.userRepo,
        s.tenantUserRepo,
        s.logger,
    )
    
    users := v1.Group("/users")
    {
        users.POST("", userHandler.CreateUser)
        users.GET("", userHandler.ListUsers)
        users.GET("/:id", userHandler.GetUser)
        users.PUT("/:id", userHandler.UpdateUser)
        users.DELETE("/:id", userHandler.DeleteUser)
        
        // User's tenant memberships
        users.GET("/:id/tenants", userHandler.GetUserTenants)
    }
    
    // Tenant management routes
    tenantHandler := handlers.NewTenantHandler(
        s.tenantRepo,
        s.userRepo,
        s.tenantUserRepo,
        s.metricsService,
        s.logsService,
        s.tracesService,
        s.logger,
    )
    
    tenants := v1.Group("/tenants")
    {
        tenants.POST("", tenantHandler.CreateTenant)
        tenants.GET("", tenantHandler.ListTenants)
        tenants.GET("/:id", tenantHandler.GetTenant)
        tenants.PUT("/:id", tenantHandler.UpdateTenant)
        tenants.DELETE("/:id", tenantHandler.DeleteTenant)
        
        // Quota management
        tenants.GET("/:id/quotas", tenantHandler.GetTenantQuotas)
        tenants.PUT("/:id/quotas", tenantHandler.UpdateTenantQuotas)
        
        // User management within tenant
        tenants.POST("/:id/users", tenantHandler.AddUserToTenant)
        tenants.GET("/:id/users", tenantHandler.ListTenantUsers)
        tenants.PUT("/:id/users/:userId", tenantHandler.UpdateUserRoles)
        tenants.DELETE("/:id/users/:userId", tenantHandler.RemoveUserFromTenant)
        tenants.GET("/:id/users/:userId", tenantHandler.GetTenantUser)
    }
    
    // Role management routes
    roleHandler := handlers.NewRoleHandler(
        s.roleRepo,
        s.tenantRepo,
        s.logger,
    )
    
    roles := v1.Group("/tenants/:tenantId/roles")
    {
        roles.POST("", roleHandler.CreateRole)
        roles.GET("", roleHandler.ListRoles)
        roles.GET("/:id", roleHandler.GetRole)
        roles.PUT("/:id", roleHandler.UpdateRole)
        roles.DELETE("/:id", roleHandler.DeleteRole)
        
        // Permission management
        roles.GET("/:id/permissions", roleHandler.GetRolePermissions)
        roles.PUT("/:id/permissions", roleHandler.UpdateRolePermissions)
    }
}
```

---

## 8. Security & Compliance

### 8.1 Data Isolation

**Enforcement Points:**
1. **API Layer:** Middleware validates tenant context
2. **Service Layer:** All Victoria* calls include AccountID
3. **Storage Layer:** Physical separation via AccountID
4. **Cache Layer:** Tenant-prefixed cache keys

```go
// Tenant-aware cache keys
func getTenantCacheKey(tenantID, key string) string {
    return fmt.Sprintf("tenant:%s:%s", tenantID, key)
}
```

### 8.2 Audit Logging

```go
// internal/models/audit.go
type AuditLog struct {
    ID         string    `json:"id"`
    TenantID   string    `json:"tenant_id"`
    UserID     string    `json:"user_id"`
    Action     string    `json:"action"`
    Resource   string    `json:"resource"`
    ResourceID string    `json:"resource_id"`
    Changes    string    `json:"changes,omitempty"`
    IPAddress  string    `json:"ip_address"`
    Timestamp  time.Time `json:"timestamp"`
    Success    bool      `json:"success"`
}
```

### 8.3 Rate Limiting

```go
// internal/api/middleware/rate_limit_tenant.go
func TenantRateLimitMiddleware(tenantRepo repo.TenantRepository) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenant := c.MustGet("tenant").(*models.Tenant)
        
        // Check rate limit based on tenant quotas
        key := fmt.Sprintf("ratelimit:tenant:%s", tenant.ID)
        
        // Implement token bucket or sliding window
        // based on tenant.Quotas.MaxQueriesPerMinute
        
        c.Next()
    }
}
```

---

## 9. Implementation Plan

### Phase 1: Foundation (Weeks 1-2)
- ✅ Create tenant data models
- ✅ Implement tenant repository (Weaviate)
- ✅ Update middleware for tenant extraction
- ✅ Add context propagation utilities

### Phase 2: Victoria* Integration (Weeks 3-4)
- ✅ Update VictoriaMetrics service with tenant methods
- ✅ Update VictoriaLogs service with tenant methods
- ✅ Update VictoriaTraces service with tenant methods
- ✅ Test AccountID header propagation

### Phase 3: API & Handlers (Week 5)
- ✅ Implement tenant CRUD handlers
- ✅ Add tenant management routes
- ✅ Update existing handlers to use tenant context

### Phase 4: Security & Validation (Week 6)
- ✅ Implement quota checking
- ✅ Add rate limiting per tenant
- ✅ Implement audit logging
- ✅ Add tenant access validation

### Phase 5: Testing & Documentation (Week 7-8)
- ⏳ Unit tests for all components
- ⏳ Integration tests
- ⏳ Load testing with multiple tenants
- ⏳ Update API documentation

---

## 10. Code Examples

### Organizations Setup
For these examples, we'll use two organizations:
- **platformbuilds** - Global Admin: aarvee
- **chikacafe** - Global Admin: Tony

**Key User:** Akhil is a Tenant Guest for platformbuilds, while Tenant Admin for chikacafe

### 10.1 Creating Global Users with Different Roles

```bash
# Step 1a: Create Global Admin for platformbuilds - aarvee
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <system_token>" \
  -d '{
    "email": "aarvee@platformbuilds.com",
    "username": "aarvee",
    "full_name": "Aarvee Platform Admin",
    "global_role": "global_admin",
    "password": "SecurePassword123!",
    "timezone": "America/New_York"
  }'
```

**Response:**
```json
{
  "status": "success",
  "user": {
    "id": "user_10001",
    "email": "aarvee@platformbuilds.com",
    "username": "aarvee",
    "full_name": "Aarvee Platform Admin",
    "global_role": "global_admin",
    "status": "active",
    "created_at": "2025-11-08T12:00:00Z"
  }
}
```

```bash
# Step 1b: Create Global Admin for chikacafe - Tony
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <system_token>" \
  -d '{
    "email": "tony@chikacafe.com",
    "username": "tony",
    "full_name": "Tony Chikacafe Admin",
    "global_role": "global_admin",
    "password": "SecurePassword123!"
  }'
```

**Response:**
```json
{
  "status": "success",
  "user": {
    "id": "user_10002",
    "email": "tony@chikacafe.com",
    "username": "tony",
    "full_name": "Tony Chikacafe Admin",
    "global_role": "global_admin",
    "status": "active",
    "created_at": "2025-11-08T12:01:00Z"
  }
}
```

```bash
# Step 1c: Create a standard Tenant User - Akhil
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin_token>" \
  -d '{
    "email": "akhil@example.com",
    "username": "akhil",
    "full_name": "Akhil Kumar",
    "global_role": "tenant_user",
    "password": "SecurePassword123!"
  }'
```

**Response:**
```json
{
  "status": "success",
  "user": {
    "id": "user_10003",
    "email": "akhil@example.com",
    "username": "akhil",
    "full_name": "Akhil Kumar",
    "global_role": "tenant_user",
    "status": "active",
    "created_at": "2025-11-08T12:02:00Z"
  }
}
```

### 10.2 Creating Tenants with Deployments

```bash
# Step 2a: Create platformbuilds tenant
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <aarvee_token>" \
  -d '{
    "name": "platformbuilds",
    "display_name": "PLATFORMBUILDS",
    "admin_email": "aarvee@platformbuilds.com",
    "deployments": {
      "metrics": {
        "endpoints": [
          "http://vm-platformbuilds-metrics-1.victoria.svc.cluster.local:8428",
          "http://vm-platformbuilds-metrics-2.victoria.svc.cluster.local:8428"
        ],
        "timeout": 30000,
        "username": "platformbuilds_user",
        "password": "secure_password",
        "namespace": "tenant-platformbuilds-metrics",
        "cluster": "prod-us-east-1",
        "health_check": {
          "enabled": true,
          "interval": 30,
          "timeout": 5
        }
      },
      "logs": {
        "endpoints": [
          "http://vl-platformbuilds-logs-1.victoria.svc.cluster.local:9428",
          "http://vl-platformbuilds-logs-2.victoria.svc.cluster.local:9428"
        ],
        "timeout": 30000,
        "namespace": "tenant-platformbuilds-logs",
        "cluster": "prod-us-east-1"
      },
      "traces": {
        "endpoints": [
          "http://vt-platformbuilds-traces-1.victoria.svc.cluster.local:7428"
        ],
        "timeout": 30000,
        "namespace": "tenant-platformbuilds-traces",
        "cluster": "prod-us-east-1"
      }
    },
    "quotas": {
      "metrics_retention_days": 90,
      "logs_retention_days": 30,
      "max_queries_per_minute": 1000
    },
    "features": {
      "unified_query_engine": true,
      "ai_root_cause_analysis": true
    }
  }'
```

**Response:**
```json
{
  "status": "success",
  "tenant": {
    "id": "platformbuilds",
    "name": "platformbuilds",
    "display_name": "PLATFORMBUILDS",
    "status": "active",
    "deployments": {
      "metrics": {
        "endpoints": [
          "http://vm-platformbuilds-metrics-1.victoria.svc.cluster.local:8428",
          "http://vm-platformbuilds-metrics-2.victoria.svc.cluster.local:8428"
        ]
      },
      "logs": { "..." },
      "traces": { "..." }
    },
    "created_at": "2025-11-08T12:03:00Z"
  }
}
```

```bash
# Step 2b: Create chikacafe tenant
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <tony_token>" \
  -d '{
    "name": "chikacafe",
    "display_name": "Chika Cafe",
    "admin_email": "tony@chikacafe.com",
    "deployments": {
      "metrics": {
        "endpoints": [
          "http://vm-chikacafe-metrics-1.victoria.svc.cluster.local:8428"
        ],
        "timeout": 30000,
        "username": "chikacafe_user",
        "password": "secure_password",
        "namespace": "tenant-chikacafe-metrics",
        "cluster": "prod-us-west-2"
      },
      "logs": {
        "endpoints": [
          "http://vl-chikacafe-logs-1.victoria.svc.cluster.local:9428"
        ],
        "timeout": 30000,
        "namespace": "tenant-chikacafe-logs",
        "cluster": "prod-us-west-2"
      },
      "traces": {
        "endpoints": [
          "http://vt-chikacafe-traces-1.victoria.svc.cluster.local:7428"
        ],
        "timeout": 30000,
        "namespace": "tenant-chikacafe-traces",
        "cluster": "prod-us-west-2"
      }
    },
    "quotas": {
      "metrics_retention_days": 60,
      "logs_retention_days": 21,
      "max_queries_per_minute": 500
    },
    "features": {
      "unified_query_engine": true,
      "ai_root_cause_analysis": false
    }
  }'
```

**Response:**
```json
{
  "status": "success",
  "tenant": {
    "id": "chikacafe",
    "name": "chikacafe",
    "display_name": "Chika Cafe",
    "status": "active",
    "deployments": {
      "metrics": {
        "endpoints": [
          "http://vm-chikacafe-metrics-1.victoria.svc.cluster.local:8428"
        ]
      },
      "logs": { "..." },
      "traces": { "..." }
    },
    "created_at": "2025-11-08T12:04:00Z"
  }
}
```

### 10.3 Assigning Users to Tenants with Tenant Roles

```bash
# Step 3a: Assign Akhil as Tenant Guest to platformbuilds
curl -X POST http://localhost:8080/api/v1/tenants/platformbuilds/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <aarvee_token>" \
  -d '{
    "user_id": "user_10003",
    "tenant_role": "tenant_guest"
  }'
```

**Response:**
```json
{
  "status": "success",
  "tenant_user": {
    "id": "tu_67890",
    "tenant_id": "platformbuilds",
    "user_id": "user_10003",
    "tenant_role": "tenant_guest",
    "status": "active",
    "joined_at": "2025-11-08T12:05:00Z",
    "invited_by": "user_10001"
  }
}
```

```bash
# Step 3b: Assign Akhil as Tenant Admin to chikacafe
curl -X POST http://localhost:8080/api/v1/tenants/chikacafe/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <tony_token>" \
  -d '{
    "user_id": "user_10003",
    "tenant_role": "tenant_admin"
  }'
```

**Response:**
```json
{
  "status": "success",
  "tenant_user": {
    "id": "tu_67891",
    "tenant_id": "chikacafe",
    "user_id": "user_10003",
    "tenant_role": "tenant_admin",
    "status": "active",
    "joined_at": "2025-11-08T12:06:00Z",
    "invited_by": "user_10002"
  }
}
```

### 10.4 Same User with Different Roles Across Tenants

**Summary:** Akhil (user_10003) now has:
- **Tenant Guest** role in platformbuilds (read-only access)
- **Tenant Admin** role in chikacafe (full admin privileges)

This demonstrates the flexibility of the two-tier role system where the same user can have different capabilities in different tenants.

### 10.5 User Login and Multi-Tenant Access

```bash
# Step 5a: Akhil logs in
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "akhil@example.com",
    "password": "SecurePassword123!"
  }'
```

**Response:**
```json
{
  "status": "success",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "user_10003",
    "email": "akhil@example.com",
    "full_name": "Akhil Kumar",
    "global_role": "tenant_user"
  },
  "global_permissions": [
    "users.read.self",
    "tenants.read"
  ],
  "accessible_tenants": [
    {
      "tenant_id": "platformbuilds",
      "tenant_name": "PLATFORMBUILDS",
      "tenant_role": "tenant_guest",
      "is_default": true,
      "status": "active"
    },
    {
      "tenant_id": "chikacafe",
      "tenant_name": "Chika Cafe",
      "tenant_role": "tenant_admin",
      "is_default": false,
      "status": "active"
    }
  ],
  "current_tenant": {
    "tenant_id": "platformbuilds",
    "tenant_role": "tenant_guest",
    "tenant_permissions": [
      "dashboards.read",
      "kpis.read",
      "alerts.read",
      "metrics.read",
      "logs.read",
      "traces.read",
      "tenant.config.read"
    ]
  }
}
```

```bash
# Step 5b: Global Admin (aarvee) logs in
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "aarvee@platformbuilds.com",
    "password": "SecurePassword123!"
  }'
```

**Response:**
```json
{
  "status": "success",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "user_10001",
    "email": "aarvee@platformbuilds.com",
    "full_name": "Aarvee Platform Admin",
    "global_role": "global_admin"
  },
  "global_permissions": [
    "platform.admin",
    "users.create",
    "users.read",
    "users.update",
    "users.delete",
    "tenants.create",
    "tenants.read",
    "tenants.update",
    "tenants.delete",
    "tenants.*.admin",
    "platform.config",
    "audit.read"
  ],
  "accessible_tenants": "all",
  "note": "Global Admin has automatic admin access to all tenants"
}
```

### 10.6 Switching Between Tenants

```bash
# Step 6: Akhil switches from platformbuilds to chikacafe
curl -X GET http://localhost:8080/api/v1/metrics/query \
  -H "Authorization: Bearer <akhil_token>" \
  -H "X-Switch-Tenant: chikacafe" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "up{job=\"api-server\"}",
    "time": "2025-11-08T12:00:00Z"
  }'

# Middleware automatically:
# 1. Validates Akhil has access to chikacafe
# 2. Loads Akhil's role in chikacafe (tenant_admin)
# 3. Routes to chikacafe's VictoriaMetrics deployment
# 4. Applies tenant_admin permissions
# 5. Returns data from chikacafe's isolated deployment
```

### 10.7 Listing User's Tenant Memberships

```bash
# Step 7: Get all tenants Akhil has access to
curl -X GET http://localhost:8080/api/v1/users/user_10003/tenants \
  -H "Authorization: Bearer <akhil_token>"
```

**Response:**
```json
{
  "status": "success",
  "tenants": [
    {
      "id": "tu_67890",
      "tenant_id": "platformbuilds",
      "tenant_name": "PLATFORMBUILDS",
      "user_id": "user_10003",
      "tenant_role": "tenant_guest",
      "status": "active",
      "joined_at": "2025-11-08T12:05:00Z"
    },
    {
      "id": "tu_67891",
      "tenant_id": "chikacafe",
      "tenant_name": "Chika Cafe",
      "user_id": "user_10003",
      "tenant_role": "tenant_admin",
      "status": "active",
      "joined_at": "2025-11-08T12:06:00Z"
    }
  ],
  "total": 2
}
```

### 10.8 Updating User's Role in a Tenant

```bash
# Step 8: Tony (Tenant Admin) promotes a user in chikacafe from guest to editor
curl -X PUT http://localhost:8080/api/v1/tenants/chikacafe/users/user_10005 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <tony_token>" \
  -H "X-Tenant-ID: chikacafe" \
  -d '{
    "tenant_role": "tenant_editor"
  }'
```

**Response:**
```json
{
  "status": "success",
  "message": "User role updated successfully",
  "tenant_user": {
    "user_id": "user_10005",
    "tenant_id": "chikacafe",
    "tenant_role": "tenant_editor",
    "updated_at": "2025-11-08T12:20:00Z"
  }
}
```

### 10.9 Permission-Based Access Control Examples

```bash
# Example 1: Akhil (Tenant Guest) tries to create dashboard in platformbuilds (DENIED)
curl -X POST http://localhost:8080/api/v1/tenants/platformbuilds/dashboards \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <akhil_token>" \
  -H "X-Tenant-ID: platformbuilds" \
  -d '{ "name": "My Dashboard" }'
```

**Response:**
```json
{
  "status": "error",
  "error": "Missing required permission: dashboards.create",
  "user_role": "tenant_guest",
  "required_permission": "dashboards.create"
}
```

```bash
# Example 2: Akhil (Tenant Admin) creates dashboard in chikacafe (ALLOWED)
curl -X POST http://localhost:8080/api/v1/tenants/chikacafe/dashboards \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <akhil_token>" \
  -H "X-Tenant-ID: chikacafe" \
  -d '{ "name": "Performance Dashboard" }'
```

**Response:**
```json
{
  "status": "success",
  "dashboard": {
    "id": "dash_001",
    "name": "Performance Dashboard",
    "tenant_id": "chikacafe",
    "created_by": "user_10003",
    "created_at": "2025-11-08T12:25:00Z"
  }
}
```

```bash
# Example 3: Tenant Editor tries to invite user (DENIED)
curl -X POST http://localhost:8080/api/v1/tenants/chikacafe/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <editor_token>" \
  -H "X-Tenant-ID: chikacafe" \
  -d '{ "user_id": "user_10006", "tenant_role": "tenant_guest" }'
```

**Response:**
```json
{
  "status": "error",
  "error": "Missing required permission: tenant.users.invite",
  "user_role": "tenant_editor",
  "note": "User management requires Tenant Admin role"
}
```

```bash
# Example 4: Tenant Admin invites user (ALLOWED)
curl -X POST http://localhost:8080/api/v1/tenants/chikacafe/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <akhil_token>" \
  -H "X-Tenant-ID: chikacafe" \
  -d '{ "user_id": "user_10006", "tenant_role": "tenant_guest" }'
```

**Response:**
```json
{
  "status": "success",
  "tenant_user": {
    "id": "tu_12345",
    "tenant_id": "chikacafe",
    "user_id": "user_10006",
    "tenant_role": "tenant_guest",
    "invited_by": "user_10003",
    "joined_at": "2025-11-08T12:30:00Z"
  }
}
```

```bash
# Example 5: aarvee (Global Admin) manages any tenant (ALLOWED)
curl -X POST http://localhost:8080/api/v1/tenants/chikacafe/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <aarvee_token>" \
  -d '{ "user_id": "user_10007", "tenant_role": "tenant_admin" }'
```

**Response:**
```json
{
  "status": "success",
  "note": "Global Admin can perform all actions in any tenant",
  "tenant_user": {
    "id": "tu_12346",
    "tenant_id": "chikacafe",
    "user_id": "user_10007",
    "tenant_role": "tenant_admin",
    "invited_by": "user_10001",
    "joined_at": "2025-11-08T12:35:00Z"
  }
}
```

### 10.9 Querying with Tenant Context and Routing

```go
// In your handler
func (h *MetricsHandler) Query(c *gin.Context) {
    // Tenant context automatically extracted by middleware
    tenant := c.MustGet("tenant").(*models.Tenant)
    
    // Create context with tenant info
    ctx := utils.WithTenantID(c.Request.Context(), tenant.ID)
    
    var request models.MetricsQLQueryRequest
    if err := c.ShouldBindJSON(&request); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Router automatically selects correct deployment for tenant
    result, err := h.metricsRouter.ExecuteQueryWithTenant(ctx, &request)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, result)
}
```

**Request Flow:**
```
1. Client Request with X-Tenant-ID: platformbuilds
   ↓
2. Middleware extracts tenant → loads tenant object from repo
   ↓
3. Router.GetServiceForTenant(platformbuilds)
   ↓
4. Check cache for platformbuilds service instance
   ↓
5. If not cached, create service with tenant's deployment endpoints:
   - http://vm-platformbuilds-metrics-1.victoria.svc.cluster.local:8428
   - http://vm-platformbuilds-metrics-2.victoria.svc.cluster.local:8428
   ↓
6. Execute query on tenant-specific deployment
   ↓
7. Return results
```

### 10.3 Multi-Tenant Query Example

```bash
# Query metrics for platformbuilds tenant
curl -X POST http://localhost:8080/api/v1/metrics/query \
  -H "X-Tenant-ID: platformbuilds" \
  -H "Authorization: Bearer <aarvee_token>" \
  -d '{
    "query": "up{job=\"api-server\"}",
    "time": "2025-11-08T12:00:00Z"
  }'

# Mirador-Core routing:
# 1. Extract tenant: platformbuilds
# 2. Load tenant deployment config
# 3. Route to: http://vm-platformbuilds-metrics-1.victoria.svc.cluster.local:8428
# 4. Execute: GET /api/v1/query?query=up{job="api-server"}
# 5. Return results
```

```bash
# Query logs for different tenant (chikacafe)
curl -X POST http://localhost:8080/api/v1/logs/query \
  -H "X-Tenant-ID: chikacafe" \
  -H "Authorization: Bearer <tony_token>" \
  -d '{
    "query": "*",
    "start": 1699430400000,
    "end": 1699434000000
  }'

# Mirador-Core routing:
# 1. Extract tenant: chikacafe
# 2. Load tenant deployment config
# 3. Route to: http://vl-chikacafe-logs-1.victoria.svc.cluster.local:9428
# 4. Execute query on chikacafe's isolated deployment
# 5. Return results
```

### 10.4 Infrastructure Setup Example

**Kubernetes Deployment per Tenant:**

```yaml
# deployment-platformbuilds-metrics.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: tenant-platformbuilds-metrics
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: vm-platformbuilds-metrics
  namespace: tenant-platformbuilds-metrics
spec:
  serviceName: vm-platformbuilds-metrics
  replicas: 2
  selector:
    matchLabels:
      app: victoriametrics
      tenant: platformbuilds
  template:
    metadata:
      labels:
        app: victoriametrics
        tenant: platformbuilds
    spec:
      containers:
      - name: victoriametrics
        image: victoriametrics/victoria-metrics:latest
        args:
        - -storageDataPath=/storage
        - -retentionPeriod=90d
        - -httpListenAddr=:8428
        ports:
        - containerPort: 8428
          name: http
        volumeMounts:
        - name: storage
          mountPath: /storage
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
  volumeClaimTemplates:
  - metadata:
      name: storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 100Gi
---
apiVersion: v1
kind: Service
metadata:
  name: vm-platformbuilds-metrics
  namespace: tenant-platformbuilds-metrics
spec:
  selector:
    app: victoriametrics
    tenant: platformbuilds
  ports:
  - port: 8428
    targetPort: 8428
  clusterIP: None  # Headless service for StatefulSet
---
# Ingress for external access (optional)
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: vm-platformbuilds-metrics-ingress
  namespace: tenant-platformbuilds-metrics
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - metrics-platformbuilds.yourdomain.com
    secretName: platformbuilds-metrics-tls
  rules:
  - host: metrics-platformbuilds.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: vm-platformbuilds-metrics
            port:
              number: 8428
```

**Similar deployments for VictoriaLogs and VictoriaTraces:**
- `deployment-platformbuilds-logs.yaml` → namespace: `tenant-platformbuilds-logs`
- `deployment-platformbuilds-traces.yaml` → namespace: `tenant-platformbuilds-traces`
- `deployment-chikacafe-metrics.yaml` → namespace: `tenant-chikacafe-metrics`
- `deployment-chikacafe-logs.yaml` → namespace: `tenant-chikacafe-logs`
- `deployment-chikacafe-traces.yaml` → namespace: `tenant-chikacafe-traces`

---

## Conclusion

This multi-tenant strategy provides:

✅ **Complete Physical Isolation:** Separate Victoria* deployments per tenant  
✅ **Intelligent Routing:** Mirador-Core routes to correct deployment based on tenant  
✅ **No Data Leakage:** Zero risk of cross-tenant data exposure  
✅ **Independent Scaling:** Each tenant deployment scales independently  
✅ **Flexible Management:** Full CRUD for tenants with deployment configuration  
✅ **High Availability:** Multiple endpoints per deployment for redundancy  
✅ **Network Isolation:** K8s namespace/cluster separation  
✅ **Compliance Ready:** Meets strictest regulatory requirements  
✅ **Operational Excellence:** Per-tenant monitoring, quotas, and resource limits

**Key Advantages Over Shared AccountID Approach:**

| Aspect | Separate Deployments ✅ | Shared with AccountID |
|--------|------------------------|----------------------|
| Data Isolation | Physical (strongest) | Logical (header-based) |
| Performance | Independent per tenant | Shared resources |
| Scaling | Per-tenant control | Cluster-wide only |
| Security | Network-level isolation | Application-level only |
| Compliance | Full regulatory support | Limited compliance |
| Noisy Neighbor | Eliminated | Possible |
| Cost Allocation | Direct per-tenant | Requires estimation |
| Blast Radius | Single tenant | All tenants |

**Architecture Benefits:**

1. **Mirador-Core as Smart Router:**
   - Centralizes tenant management
   - Provides unified API across all tenants
   - Handles authentication, authorization, and routing
   - Caches tenant service instances for performance

2. **Operational Flexibility:**
   - Deploy tenants in different regions/clusters
   - Different resource allocations per tenant tier
   - Independent version upgrades per tenant
   - Tenant-specific backup/restore

3. **Enterprise-Grade Security:**
   - Complete network isolation
   - No shared infrastructure attack surface
   - Tenant-specific access credentials
   - Audit trail per tenant

## 11. Role Capabilities Matrix

### 11.1 Global Roles Capabilities

| Capability | Global Admin | Global Tenant Admin | Tenant User |
|-----------|--------------|---------------------|-------------|
| **Platform Management** |
| Platform configuration | ✅ | ❌ | ❌ |
| View audit logs | ✅ | ❌ | ❌ |
| **User Management** |
| Create users | ✅ | ❌ | ❌ |
| Edit any user | ✅ | ❌ | ❌ |
| Delete any user | ✅ | ❌ | ❌ |
| View all users | ✅ | ✅ (for assignment) | ❌ |
| Edit own profile | ✅ | ✅ | ✅ |
| **Tenant Management** |
| Create tenants | ✅ | ✅ | ❌ |
| Edit any tenant | ✅ | ✅ | ❌ |
| Delete any tenant | ✅ | ✅ | ❌ |
| View all tenants | ✅ | ✅ | ❌ |
| View assigned tenants | ✅ | ✅ | ✅ |
| **Tenant Access** |
| Admin access to all tenants | ✅ (automatic) | ❌ | ❌ |
| Assign users to tenants | ✅ | ✅ | ❌ |

### 11.2 Tenant Roles Capabilities

| Capability | Tenant Admin | Tenant Editor | Tenant Guest |
|-----------|--------------|---------------|--------------|
| **User Management (within tenant)** |
| Invite users | ✅ | ❌ | ❌ |
| Remove users | ✅ | ❌ | ❌ |
| Update user roles | ✅ | ❌ | ❌ |
| List tenant users | ✅ | ❌ | ❌ |
| **Resource Management** |
| Create dashboards | ✅ | ✅ | ❌ |
| Edit dashboards | ✅ | ✅ | ❌ |
| Delete dashboards | ✅ | ✅ | ❌ |
| View dashboards | ✅ | ✅ | ✅ |
| Create KPIs | ✅ | ✅ | ❌ |
| Edit KPIs | ✅ | ✅ | ❌ |
| Delete KPIs | ✅ | ✅ | ❌ |
| View KPIs | ✅ | ✅ | ✅ |
| Create alerts | ✅ | ✅ | ❌ |
| Edit alerts | ✅ | ✅ | ❌ |
| Delete alerts | ✅ | ✅ | ❌ |
| View alerts | ✅ | ✅ | ✅ |
| **Data Access** |
| Query metrics | ✅ (read/write) | ✅ (read/write) | ✅ (read-only) |
| Query logs | ✅ (read/write) | ✅ (read/write) | ✅ (read-only) |
| Query traces | ✅ (read/write) | ✅ (read/write) | ✅ (read-only) |
| **Configuration** |
| Update tenant config | ✅ | ❌ | ❌ |
| View tenant config | ✅ | ✅ | ✅ |
| **Roles Management** |
| Create custom roles | ✅ | ❌ | ❌ |
| Edit custom roles | ✅ | ❌ | ❌ |
| Delete custom roles | ✅ | ❌ | ❌ |
| View roles | ✅ | ✅ | ❌ |

### 11.3 Permission Strings Reference

**Global Permissions:**
```
platform.admin              - Full platform control
users.create               - Create new users
users.read                 - View all users
users.update               - Edit any user
users.delete               - Delete users
users.read.self            - View own profile
tenants.create             - Create new tenants
tenants.read               - View tenants
tenants.update             - Edit tenants
tenants.delete             - Delete tenants
tenants.*.admin            - Admin in all tenants
tenants.users.manage       - Assign users to tenants
platform.config            - Platform configuration
audit.read                 - View audit logs
```

**Tenant Permissions:**
```
tenant.users.invite        - Invite users to tenant
tenant.users.remove        - Remove users from tenant
tenant.users.roles.update  - Update user roles in tenant
tenant.users.list          - List tenant users
tenant.config.update       - Update tenant configuration
tenant.config.read         - View tenant configuration

dashboards.create          - Create dashboards
dashboards.read            - View dashboards
dashboards.update          - Edit dashboards
dashboards.delete          - Delete dashboards

kpis.create               - Create KPIs
kpis.read                 - View KPIs
kpis.update               - Edit KPIs
kpis.delete               - Delete KPIs

alerts.create             - Create alerts
alerts.read               - View alerts
alerts.update             - Edit alerts
alerts.delete             - Delete alerts

metrics.read              - Query metrics (read)
metrics.write             - Ingest metrics (write)
logs.read                 - Query logs (read)
logs.write                - Ingest logs (write)
traces.read               - Query traces (read)
traces.write              - Ingest traces (write)

roles.create              - Create custom roles
roles.read                - View roles
roles.update              - Edit roles
roles.delete              - Delete roles
```

---

## 12. Deployment Management

### 11.1 Tenant Provisioning Workflow

```go
// internal/services/tenant_provisioner.go
package services

import (
    "context"
    "fmt"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/repo"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// TenantProvisioner handles tenant infrastructure provisioning
type TenantProvisioner struct {
    tenantRepo     repo.TenantRepository
    k8sClient      K8sClient // Interface for K8s operations
    helmClient     HelmClient // Interface for Helm operations
    logger         logger.Logger
}

// ProvisionTenant creates infrastructure for new tenant
func (p *TenantProvisioner) ProvisionTenant(
    ctx context.Context,
    tenant *models.Tenant,
    tier string, // "basic", "premium", "enterprise"
) error {
    p.logger.Info("Starting tenant provisioning",
        "tenant_id", tenant.ID,
        "tier", tier)
    
    // 1. Create K8s namespaces
    namespaces := []string{
        fmt.Sprintf("tenant-%s-metrics", tenant.Name),
        fmt.Sprintf("tenant-%s-logs", tenant.Name),
        fmt.Sprintf("tenant-%s-traces", tenant.Name),
    }
    
    for _, ns := range namespaces {
        if err := p.k8sClient.CreateNamespace(ctx, ns); err != nil {
            return fmt.Errorf("failed to create namespace %s: %w", ns, err)
        }
    }
    
    // 2. Deploy VictoriaMetrics via Helm
    metricsConfig := p.getMetricsConfig(tenant, tier)
    metricsEndpoints, err := p.helmClient.Install(ctx, HelmRelease{
        Name:      fmt.Sprintf("vm-%s", tenant.Name),
        Namespace: namespaces[0],
        Chart:     "victoria-metrics-single",
        Version:   "0.9.0",
        Values:    metricsConfig,
    })
    if err != nil {
        return fmt.Errorf("failed to deploy metrics: %w", err)
    }
    
    // 3. Deploy VictoriaLogs via Helm
    logsConfig := p.getLogsConfig(tenant, tier)
    logsEndpoints, err := p.helmClient.Install(ctx, HelmRelease{
        Name:      fmt.Sprintf("vl-%s", tenant.Name),
        Namespace: namespaces[1],
        Chart:     "victoria-logs-single",
        Version:   "0.5.0",
        Values:    logsConfig,
    })
    if err != nil {
        return fmt.Errorf("failed to deploy logs: %w", err)
    }
    
    // 4. Deploy VictoriaTraces
    tracesConfig := p.getTracesConfig(tenant, tier)
    tracesEndpoints, err := p.helmClient.Install(ctx, HelmRelease{
        Name:      fmt.Sprintf("vt-%s", tenant.Name),
        Namespace: namespaces[2],
        Chart:     "victoria-traces",
        Version:   "0.1.0",
        Values:    tracesConfig,
    })
    if err != nil {
        return fmt.Errorf("failed to deploy traces: %w", err)
    }
    
    // 5. Update tenant with deployment endpoints
    tenant.Deployments = models.TenantDeployments{
        Metrics: models.DeploymentConfig{
            Endpoints:  metricsEndpoints,
            Timeout:    30000,
            Namespace:  namespaces[0],
        },
        Logs: models.DeploymentConfig{
            Endpoints:  logsEndpoints,
            Timeout:    30000,
            Namespace:  namespaces[1],
        },
        Traces: models.DeploymentConfig{
            Endpoints:  tracesEndpoints,
            Timeout:    30000,
            Namespace:  namespaces[2],
        },
    }
    
    // 6. Save tenant with deployment info
    if err := p.tenantRepo.UpdateTenant(ctx, tenant); err != nil {
        return fmt.Errorf("failed to update tenant: %w", err)
    }
    
    p.logger.Info("Tenant provisioning completed",
        "tenant_id", tenant.ID,
        "metrics_endpoints", metricsEndpoints,
        "logs_endpoints", logsEndpoints,
        "traces_endpoints", tracesEndpoints)
    
    return nil
}

func (p *TenantProvisioner) getMetricsConfig(tenant *models.Tenant, tier string) map[string]interface{} {
    config := map[string]interface{}{
        "server": map[string]interface{}{
            "retentionPeriod": fmt.Sprintf("%dd", tenant.Quotas.MetricsRetentionDays),
        },
    }
    
    switch tier {
    case "enterprise":
        config["server"].(map[string]interface{})["resources"] = map[string]interface{}{
            "requests": map[string]string{"memory": "8Gi", "cpu": "4"},
            "limits":   map[string]string{"memory": "16Gi", "cpu": "8"},
        }
        config["server"].(map[string]interface{})["replicaCount"] = 3
    case "premium":
        config["server"].(map[string]interface{})["resources"] = map[string]interface{}{
            "requests": map[string]string{"memory": "4Gi", "cpu": "2"},
            "limits":   map[string]string{"memory": "8Gi", "cpu": "4"},
        }
        config["server"].(map[string]interface{})["replicaCount"] = 2
    default: // basic
        config["server"].(map[string]interface{})["resources"] = map[string]interface{}{
            "requests": map[string]string{"memory": "2Gi", "cpu": "1"},
            "limits":   map[string]string{"memory": "4Gi", "cpu": "2"},
        }
        config["server"].(map[string]interface{})["replicaCount"] = 1
    }
    
    return config
}
```

### 11.2 Health Monitoring

```go
// internal/services/deployment_health.go
package services

import (
    "context"
    "sync"
    "time"
    "github.com/platformbuilds/mirador-core/internal/models"
)

// DeploymentHealthMonitor monitors tenant deployment health
type DeploymentHealthMonitor struct {
    metricsRouter *VictoriaMetricsRouter
    logsRouter    *VictoriaLogsRouter
    tracesRouter  *VictoriaTracesRouter
    tenantRepo    repo.TenantRepository
    logger        logger.Logger
    
    // Health status cache
    healthStatus map[string]*TenantHealth
    mu           sync.RWMutex
}

type TenantHealth struct {
    TenantID      string
    MetricsHealth DeploymentHealth
    LogsHealth    DeploymentHealth
    TracesHealth  DeploymentHealth
    LastChecked   time.Time
}

type DeploymentHealth struct {
    Status       string // "healthy", "degraded", "unhealthy"
    Endpoints    []EndpointHealth
    ErrorCount   int
    LastError    string
}

type EndpointHealth struct {
    URL        string
    Status     string
    Latency    time.Duration
    LastCheck  time.Time
}

// StartMonitoring begins health check loop
func (m *DeploymentHealthMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.checkAllTenants(ctx)
        }
    }
}

func (m *DeploymentHealthMonitor) checkAllTenants(ctx context.Context) {
    tenants, _, err := m.tenantRepo.ListTenants(ctx, 1000, 0)
    if err != nil {
        m.logger.Error("Failed to list tenants for health check", "error", err)
        return
    }
    
    var wg sync.WaitGroup
    for _, tenant := range tenants {
        if tenant.Status != models.TenantStatusActive {
            continue
        }
        
        wg.Add(1)
        go func(t *models.Tenant) {
            defer wg.Done()
            m.checkTenantHealth(ctx, t)
        }(tenant)
    }
    wg.Wait()
}

func (m *DeploymentHealthMonitor) checkTenantHealth(ctx context.Context, tenant *models.Tenant) {
    health := &TenantHealth{
        TenantID:    tenant.ID,
        LastChecked: time.Now(),
    }
    
    // Check metrics deployment
    if err := m.metricsRouter.HealthCheckWithTenant(ctx, tenant.ID); err != nil {
        health.MetricsHealth.Status = "unhealthy"
        health.MetricsHealth.LastError = err.Error()
        health.MetricsHealth.ErrorCount++
    } else {
        health.MetricsHealth.Status = "healthy"
    }
    
    // Check logs deployment
    logsCtx := utils.WithTenantID(ctx, tenant.ID)
    logsSvc, err := m.logsRouter.GetServiceForTenant(logsCtx, tenant.ID)
    if err != nil {
        health.LogsHealth.Status = "unhealthy"
        health.LogsHealth.LastError = err.Error()
    } else if err := logsSvc.HealthCheck(logsCtx); err != nil {
        health.LogsHealth.Status = "unhealthy"
        health.LogsHealth.LastError = err.Error()
    } else {
        health.LogsHealth.Status = "healthy"
    }
    
    // Check traces deployment
    tracesCtx := utils.WithTenantID(ctx, tenant.ID)
    tracesSvc, err := m.tracesRouter.GetServiceForTenant(tracesCtx, tenant.ID)
    if err != nil {
        health.TracesHealth.Status = "unhealthy"
        health.TracesHealth.LastError = err.Error()
    } else {
        // VictoriaTraces health check
        health.TracesHealth.Status = "healthy"
    }
    
    // Update cache
    m.mu.Lock()
    m.healthStatus[tenant.ID] = health
    m.mu.Unlock()
    
    // Log unhealthy deployments
    if health.MetricsHealth.Status != "healthy" ||
       health.LogsHealth.Status != "healthy" ||
       health.TracesHealth.Status != "healthy" {
        m.logger.Warn("Tenant deployment unhealthy",
            "tenant_id", tenant.ID,
            "metrics", health.MetricsHealth.Status,
            "logs", health.LogsHealth.Status,
            "traces", health.TracesHealth.Status)
    }
}

// GetTenantHealth returns cached health status
func (m *DeploymentHealthMonitor) GetTenantHealth(tenantID string) *TenantHealth {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.healthStatus[tenantID]
}
```

---

**Next Steps:**
1. Review and approve this routing-based strategy
2. Set up tenant provisioning infrastructure (K8s + Helm)
3. Implement router services with caching
4. Begin Phase 1 implementation (tenant model + repository)
5. Deploy sample tenant deployments for testing
6. Implement health monitoring and alerting
7. Create tenant migration/onboarding automation

---

**Appendix A: Comparison with AccountID Approach**

We moved from the AccountID-based approach to separate deployments because:

1. **Stronger Isolation:** No risk of header spoofing or misconfiguration
2. **Better Performance:** No resource contention between tenants
3. **Simpler Operations:** Each tenant is independently manageable
4. **Compliance:** Meets regulatory requirements for data isolation
5. **Cost Transparency:** Direct cost allocation per tenant
6. **Flexibility:** Different configs/versions per tenant

**Appendix B: Migration Path**

For existing deployments using AccountID:
1. Create new tenant-specific deployments
2. Migrate data using Victoria* export/import tools
3. Update tenant configuration in Mirador-Core
4. Gradually switch routing to new deployments
5. Decommission shared deployments

This approach provides zero-downtime migration with rollback capability.
