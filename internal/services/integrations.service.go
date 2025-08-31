// ================================
// internal/services/integrations.service.go - External Integrations
// ================================

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/platformbuilds/miradorstack/internal/config"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type IntegrationsService struct {
	config config.IntegrationsConfig
	client *http.Client
	logger logger.Logger
}

type IntegrationsConfig struct {
	Slack struct {
		WebhookURL string `json:"webhook_url"`
		Channel    string `json:"channel"`
		Enabled    bool   `json:"enabled"`
	} `json:"slack"`

	MSTeams struct {
		WebhookURL string `json:"webhook_url"`
		Enabled    bool   `json:"enabled"`
	} `json:"ms_teams"`

	Email struct {
		SMTPHost    string `json:"smtp_host"`
		SMTPPort    int    `json:"smtp_port"`
		FromAddress string `json:"from_address"`
		Enabled     bool   `json:"enabled"`
	} `json:"email"`
}

func NewIntegrationsService(cfg config.IntegrationsConfig, logger logger.Logger) *IntegrationsService {
	return &IntegrationsService{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// SendSlackNotification sends alerts and predictions to Slack
func (s *IntegrationsService) SendSlackNotification(ctx context.Context, notification *models.Notification) error {
	if !s.config.Slack.Enabled {
		return nil
	}

	slackPayload := map[string]interface{}{
		"channel": s.config.Slack.Channel,
		"attachments": []map[string]interface{}{
			{
				"color":     s.getSlackColor(notification.Severity),
				"title":     notification.Title,
				"text":      notification.Message,
				"timestamp": notification.Timestamp.Unix(),
				"fields": []map[string]interface{}{
					{
						"title": "Component",
						"value": notification.Component,
						"short": true,
					},
					{
						"title": "Severity",
						"value": notification.Severity,
						"short": true,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(slackPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.Slack.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack notification failed with status %d", resp.StatusCode)
	}

	s.logger.Info("Slack notification sent", "type", notification.Type, "component", notification.Component)
	return nil
}

// SendMSTeamsNotification sends to MS Teams
func (s *IntegrationsService) SendMSTeamsNotification(ctx context.Context, notification *models.Notification) error {
	if !s.config.MSTeams.Enabled {
		return nil
	}

	teamsPayload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"summary":    notification.Title,
		"themeColor": s.getTeamsColor(notification.Severity),
		"sections": []map[string]interface{}{
			{
				"activityTitle":    notification.Title,
				"activitySubtitle": notification.Component,
				"text":             notification.Message,
				"facts": []map[string]interface{}{
					{"name": "Severity", "value": notification.Severity},
					{"name": "Time", "value": notification.Timestamp.Format(time.RFC3339)},
					{"name": "Type", "value": notification.Type},
				},
			},
		},
	}

	jsonData, err := json.Marshal(teamsPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.MSTeams.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MS Teams notification failed with status %d", resp.StatusCode)
	}

	s.logger.Info("MS Teams notification sent", "type", notification.Type, "component", notification.Component)
	return nil
}

func (s *IntegrationsService) getSlackColor(severity string) string {
	switch severity {
	case "critical":
		return "danger"
	case "warning":
		return "warning"
	case "info":
		return "good"
	default:
		return "#439FE0"
	}
}

func (s *IntegrationsService) getTeamsColor(severity string) string {
	switch severity {
	case "critical":
		return "FF0000"
	case "warning":
		return "FFA500"
	case "info":
		return "00FF00"
	default:
		return "0078D4"
	}
}
