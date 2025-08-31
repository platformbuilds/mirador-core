package services

import (
	"context"
	"time"

	"github.com/mirador/core/internal/config"
	"github.com/mirador/core/internal/models"
	"github.com/mirador/core/pkg/logger"
)

type NotificationService struct {
	integrations *IntegrationsService
	logger       logger.Logger
}

func NewNotificationService(cfg config.IntegrationsConfig, logger logger.Logger) *NotificationService {
	return &NotificationService{
		integrations: NewIntegrationsService(cfg, logger),
		logger:       logger,
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

// ProcessPredictionNotification handles AI prediction notifications
func (s *NotificationService) ProcessPredictionNotification(ctx context.Context, prediction *models.SystemFracture) error {
	notification := &models.Notification{
		ID:        fmt.Sprintf("pred-%s", prediction.ID),
		Type:      "prediction",
		Title:     fmt.Sprintf("System Fracture Predicted: %s", prediction.Component),
		Message:   fmt.Sprintf("Component %s has %0.1f%% probability of fracture in %s. Severity: %s", 
			prediction.Component, 
			prediction.Probability*100, 
			prediction.TimeToFracture.String(),
			prediction.Severity),
		Component: prediction.Component,
		Severity:  prediction.Severity,
		Timestamp: time.Now(),
	}

	return s.SendNotification(ctx, notification)
}

// ProcessCorrelationNotification handles RCA correlation notifications
func (s *NotificationService) ProcessCorrelationNotification(ctx context.Context, correlation *models.CorrelationResult) error {
	notification := &models.Notification{
		ID:        fmt.Sprintf("rca-%s", correlation.CorrelationID),
		Type:      "correlation",
		Title:     fmt.Sprintf("Root Cause Found: %s", correlation.IncidentID),
		Message:   fmt.Sprintf("Root cause identified: %s (Confidence: %0.1f%%). Affected services: %v",
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
