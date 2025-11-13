package services

import (
	"context"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/repo"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// DataSeeder handles seeding default data into Weaviate
type DataSeeder struct {
	repo   repo.KPIRepo
	logger logger.Logger
}

// NewDataSeeder creates a new data seeder
func NewDataSeeder(repo repo.KPIRepo, logger logger.Logger) *DataSeeder {
	return &DataSeeder{
		repo:   repo,
		logger: logger,
	}
}

// SeedDefaultDashboard creates the default dashboard for a tenant
func (ds *DataSeeder) SeedDefaultDashboard(ctx context.Context, tenantID string) error {
	ds.logger.Info("Seeding default dashboard", "tenant_id", tenantID)

	// Check if default dashboard already exists
	existing, err := ds.repo.GetDashboard(ctx, tenantID, "default")
	if err == nil && existing != nil {
		ds.logger.Info("Default dashboard already exists", "tenant_id", tenantID, "dashboard_id", existing.ID)
		return nil
	}

	// Create default dashboard
	now := time.Now()
	dashboard := &models.Dashboard{
		ID:          "default",
		Name:        "Default Dashboard",
		OwnerUserID: "system",
		Visibility:  "org",
		IsDefault:   true,
		TenantID:    tenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := ds.repo.UpsertDashboard(ctx, dashboard); err != nil {
		return fmt.Errorf("failed to create default dashboard: %w", err)
	}

	ds.logger.Info("Successfully seeded default dashboard", "tenant_id", tenantID, "dashboard_id", dashboard.ID)
	return nil
}

// SeedSampleKPIs creates sample KPI definitions for demonstration
func (ds *DataSeeder) SeedSampleKPIs(ctx context.Context, tenantID string) error {
	ds.logger.Info("Seeding sample KPIs", "tenant_id", tenantID)

	sampleKPIs := ds.getSampleKPIs(tenantID)

	for _, kpi := range sampleKPIs {
		// Check if KPI already exists
		existing, err := ds.repo.GetKPI(ctx, tenantID, kpi.ID)
		if err == nil && existing != nil {
			ds.logger.Info("KPI already exists, skipping", "tenant_id", tenantID, "kpi_id", kpi.ID)
			continue
		}

		if err := ds.repo.UpsertKPI(ctx, kpi); err != nil {
			return fmt.Errorf("failed to create KPI %s: %w", kpi.ID, err)
		}

		ds.logger.Info("Created sample KPI", "tenant_id", tenantID, "kpi_id", kpi.ID, "name", kpi.Name)
	}

	ds.logger.Info("Successfully seeded sample KPIs", "tenant_id", tenantID)
	return nil
}

// SeedDefaultUserPreferences creates default user preferences for a user
func (ds *DataSeeder) SeedDefaultUserPreferences(ctx context.Context, tenantID, userID string) error {
	ds.logger.Info("Seeding default user preferences", "tenant_id", tenantID, "user_id", userID)

	// Note: User preferences are currently stored in cache, not Weaviate
	// This is a placeholder for when they might be moved to Weaviate
	// For now, we'll just log that this would create default preferences

	ds.logger.Info("Default user preferences seeding placeholder - currently stored in cache", "tenant_id", tenantID, "user_id", userID)
	return nil
}

// getSampleKPIs returns a list of sample KPI definitions
func (ds *DataSeeder) getSampleKPIs(tenantID string) []*models.KPIDefinition {
	now := time.Now()

	return []*models.KPIDefinition{
		{
			ID:     "http_request_duration",
			Kind:   "tech",
			Name:   "HTTP Request Duration",
			Unit:   "seconds",
			Format: "duration",
			Query: map[string]interface{}{
				"metric": "http_request_duration_seconds",
				"labels": map[string]interface{}{
					"method": "{{method}}",
					"status": "{{status}}",
				},
			},
			Thresholds: []models.Threshold{
				{
					Operator:    "gt",
					Value:       1.0,
					Level:       "warning",
					Description: "Request duration is high",
				},
				{
					Operator:    "gt",
					Value:       5.0,
					Level:       "critical",
					Description: "Request duration is critically high",
				},
			},
			Tags:       []string{"http", "performance", "latency"},
			Definition: "Average HTTP request duration across all endpoints",
			Sentiment:  "NEGATIVE",
			Sparkline: map[string]interface{}{
				"type": "line",
				"query": map[string]interface{}{
					"range": "1h",
				},
			},
			OwnerUserID: "system",
			Visibility:  "org",
			TenantID:    tenantID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:     "error_rate",
			Kind:   "tech",
			Name:   "Error Rate",
			Unit:   "percent",
			Format: "percentage",
			Query: map[string]interface{}{
				"metric": "http_requests_total",
				"labels": map[string]interface{}{
					"status": ">=400",
				},
				"aggregation": "rate",
			},
			Thresholds: []models.Threshold{
				{
					Operator:    "gt",
					Value:       5.0,
					Level:       "warning",
					Description: "Error rate is elevated",
				},
				{
					Operator:    "gt",
					Value:       10.0,
					Level:       "critical",
					Description: "Error rate is critically high",
				},
			},
			Tags:       []string{"errors", "reliability", "http"},
			Definition: "Percentage of HTTP requests that result in errors (4xx/5xx)",
			Sentiment:  "NEGATIVE",
			Sparkline: map[string]interface{}{
				"type": "area",
				"query": map[string]interface{}{
					"range": "1h",
				},
			},
			OwnerUserID: "system",
			Visibility:  "org",
			TenantID:    tenantID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:     "user_satisfaction",
			Kind:   "business",
			Name:   "User Satisfaction Score",
			Unit:   "points",
			Format: "number",
			Query: map[string]interface{}{
				"metric":      "user_satisfaction_score",
				"aggregation": "avg",
			},
			Thresholds: []models.Threshold{
				{
					Operator:    "lt",
					Value:       7.0,
					Level:       "warning",
					Description: "User satisfaction is below target",
				},
				{
					Operator:    "lt",
					Value:       5.0,
					Level:       "critical",
					Description: "User satisfaction is critically low",
				},
			},
			Tags:       []string{"business", "satisfaction", "users"},
			Definition: "Average user satisfaction score from feedback surveys",
			Sentiment:  "POSITIVE",
			Sparkline: map[string]interface{}{
				"type": "line",
				"query": map[string]interface{}{
					"range": "24h",
				},
			},
			OwnerUserID: "system",
			Visibility:  "org",
			TenantID:    tenantID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:     "revenue_per_user",
			Kind:   "business",
			Name:   "Revenue Per User",
			Unit:   "currency",
			Format: "currency",
			Query: map[string]interface{}{
				"metric": "revenue_total",
				"labels": map[string]interface{}{
					"type": "subscription",
				},
				"aggregation": "avg_per_user",
			},
			Thresholds: []models.Threshold{
				{
					Operator:    "lt",
					Value:       50.0,
					Level:       "warning",
					Description: "Revenue per user is below target",
				},
			},
			Tags:       []string{"business", "revenue", "users"},
			Definition: "Average monthly revenue per active user",
			Sentiment:  "POSITIVE",
			Sparkline: map[string]interface{}{
				"type": "bar",
				"query": map[string]interface{}{
					"range": "30d",
				},
			},
			OwnerUserID: "system",
			Visibility:  "org",
			TenantID:    tenantID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:     "system_uptime",
			Kind:   "tech",
			Name:   "System Uptime",
			Unit:   "percent",
			Format: "percentage",
			Query: map[string]interface{}{
				"metric":      "up",
				"aggregation": "avg",
			},
			Thresholds: []models.Threshold{
				{
					Operator:    "lt",
					Value:       99.9,
					Level:       "warning",
					Description: "System uptime is below SLA",
				},
				{
					Operator:    "lt",
					Value:       99.0,
					Level:       "critical",
					Description: "System uptime is critically low",
				},
			},
			Tags:       []string{"reliability", "uptime", "sla"},
			Definition: "Percentage of time the system has been operational",
			Sentiment:  "POSITIVE",
			Sparkline: map[string]interface{}{
				"type": "line",
				"query": map[string]interface{}{
					"range": "7d",
				},
			},
			OwnerUserID: "system",
			Visibility:  "org",
			TenantID:    tenantID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

// SeedAll performs all seeding operations for a tenant
func (ds *DataSeeder) SeedAll(ctx context.Context, tenantID string) error {
	ds.logger.Info("Starting complete data seeding", "tenant_id", tenantID)

	// Seed default dashboard
	if err := ds.SeedDefaultDashboard(ctx, tenantID); err != nil {
		return fmt.Errorf("failed to seed default dashboard: %w", err)
	}

	// Seed sample KPIs
	if err := ds.SeedSampleKPIs(ctx, tenantID); err != nil {
		return fmt.Errorf("failed to seed sample KPIs: %w", err)
	}

	// Note: User preferences seeding is a placeholder since they're stored in cache

	ds.logger.Info("Completed complete data seeding", "tenant_id", tenantID)
	return nil
}
