package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
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
		Username    string `json:"username"`
		Password    string `json:"password"`
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

// SendSlackNotification sends alerts and notifications to Slack
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
		return fmt.Errorf("slack notification failed with status %d", resp.StatusCode)
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
		return fmt.Errorf("ms teams notification failed with status %d", resp.StatusCode)
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

// SendEmailNotification sends an email alert using SMTP with optional auth.
func (s *IntegrationsService) SendEmailNotification(ctx context.Context, notification *models.Notification) error {
	if !s.config.Email.Enabled {
		return nil
	}
	if s.config.Email.SMTPHost == "" || s.config.Email.SMTPPort == 0 || s.config.Email.FromAddress == "" {
		return fmt.Errorf("email integration not properly configured")
	}

	recipients := []string{s.config.Email.FromAddress} // fallback recipient
	// TODO: if your Notification struct has To/Recipients, replace with that.

	addr := fmt.Sprintf("%s:%d", s.config.Email.SMTPHost, s.config.Email.SMTPPort)

	safeFrom, err := sanitizeEmailHeader("from address", s.config.Email.FromAddress)
	if err != nil {
		return err
	}
	if safeFrom == "" {
		return fmt.Errorf("from address cannot be empty")
	}

	safeRecipients := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		safeRecipient, err := sanitizeEmailHeader("recipient", recipient)
		if err != nil {
			return err
		}
		if safeRecipient == "" {
			return fmt.Errorf("recipient cannot be empty")
		}
		safeRecipients = append(safeRecipients, safeRecipient)
	}

	safeSeverity, err := sanitizeEmailHeader("severity", notification.Severity)
	if err != nil {
		return err
	}
	safeTitle, err := sanitizeEmailHeader("title", notification.Title)
	if err != nil {
		return err
	}
	safeComponent, err := sanitizeEmailHeader("component", notification.Component)
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("[Mirador] %s - %s", strings.ToUpper(safeSeverity), safeTitle)
	body := fmt.Sprintf(
		"Component: %s\nSeverity: %s\nTime: %s\nType: %s\n\n%s",
		safeComponent,
		safeSeverity,
		notification.Timestamp.Format(time.RFC3339),
		notification.Type,
		notification.Message,
	)

	var msgBuilder strings.Builder
	msgBuilder.WriteString("From: ")
	msgBuilder.WriteString(safeFrom)
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString("To: ")
	msgBuilder.WriteString(strings.Join(safeRecipients, ","))
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString("Subject: ")
	msgBuilder.WriteString(subject)
	msgBuilder.WriteString("\r\n\r\n")
	msgBuilder.WriteString(body)

	msg := []byte(msgBuilder.String())

	// Build auth only if username/password provided
	var auth smtp.Auth
	if s.config.Email.Username != "" && s.config.Email.Password != "" {
		auth = smtp.PlainAuth(
			"",
			s.config.Email.Username,
			s.config.Email.Password,
			s.config.Email.SMTPHost,
		)
	}

	if err := smtp.SendMail(addr, auth, safeFrom, safeRecipients, msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Email notification sent",
		"type", notification.Type,
		"component", notification.Component,
		"to", safeRecipients,
	)
	return nil
}

// sanitizeEmailHeader rejects header values that could break out of email headers.
func sanitizeEmailHeader(fieldName, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if strings.ContainsAny(trimmed, "\r\n") {
		return "", fmt.Errorf("%s contains invalid newline characters", fieldName)
	}
	return trimmed, nil
}
