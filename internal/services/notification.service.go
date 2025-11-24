package services

import (
	"context"
	"fmt"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/logging"
	"github.com/platformbuilds/mirador-core/internal/metrics"
	"github.com/platformbuilds/mirador-core/internal/models"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

type NotificationService struct {
	integrations *IntegrationsService
	logger       logging.Logger
}

func NewNotificationService(cfg config.IntegrationsConfig, logger corelogger.Logger) *NotificationService {
	return &NotificationService{
		integrations: NewIntegrationsService(cfg, logger),
		logger:       logging.FromCoreLogger(logger),
	}
}

// SendNotification dispatches notifications to configured integrations
func (s *NotificationService) SendNotification(ctx context.Context, notification *models.Notification) error {
	var errors []error

	// Send to Slack if enabled
	if err := s.integrations.SendSlackNotification(ctx, notification); err != nil {
		s.logger.Error("Slack notification failed", "error", err)
		errors = append(errors, err)
		metrics.NotificationsSent.WithLabelValues("slack", notification.Type, "false").Inc()
	} else {
		metrics.NotificationsSent.WithLabelValues("slack", notification.Type, "true").Inc()
	}

	// Send to MS Teams if enabled
	if err := s.integrations.SendMSTeamsNotification(ctx, notification); err != nil {
		s.logger.Error("MS Teams notification failed", "error", err)
		errors = append(errors, err)
		metrics.NotificationsSent.WithLabelValues("teams", notification.Type, "false").Inc()
	} else {
		metrics.NotificationsSent.WithLabelValues("teams", notification.Type, "true").Inc()
	}

	// Send email notification
	if err := s.integrations.SendEmailNotification(ctx, notification); err != nil {
		s.logger.Error("Email notification failed", "error", err)
		errors = append(errors, err)
		metrics.NotificationsSent.WithLabelValues("email", notification.Type, "false").Inc()
	} else {
		metrics.NotificationsSent.WithLabelValues("email", notification.Type, "true").Inc()
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification partially failed: %d/%d integrations failed", len(errors), 3)
	}

	return nil
}

// ProcessCorrelationNotification handles RCA correlation notifications
func (s *NotificationService) ProcessCorrelationNotification(ctx context.Context, correlation *models.CorrelationResult) error {
	notification := &models.Notification{
		ID:    fmt.Sprintf("rca-%s", correlation.CorrelationID),
		Type:  "correlation",
		Title: fmt.Sprintf("Root Cause Found: %s", correlation.IncidentID),
		Message: fmt.Sprintf("Root cause identified: %s (Confidence: %0.1f%%). Affected services: %v",
			correlation.RootCause,
			correlation.Confidence*100,
			correlation.AffectedServices),
		Component: "rca-engine",
		Severity:  determineSeverityFromConfidence(correlation.Confidence),
		Timestamp: time.Now(),
	}

	return s.SendNotification(ctx, notification)
}

func determineSeverityFromConfidence(confidence float64) string {
	if confidence >= 0.9 {
		return "high"
	} else if confidence >= 0.7 {
		return "medium"
	}
	return "low"
}
