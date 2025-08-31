package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// ValidateEndpoint validates that an endpoint is properly formatted
func ValidateEndpoint(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}

	// Parse as URL
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Check scheme
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("endpoint must use http or https scheme")
	}

	// Check host
	if parsed.Host == "" {
		return fmt.Errorf("endpoint must include host")
	}

	return nil
}

// ValidateGRPCEndpoint validates gRPC endpoint format
func ValidateGRPCEndpoint(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("gRPC endpoint cannot be empty")
	}

	// Check if it contains a port
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("gRPC endpoint must include port: %w", err)
	}

	// Validate host
	if host == "" {
		return fmt.Errorf("gRPC endpoint must include host")
	}

	// Validate port range
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port number must be between 1 and 65535")
	}

	return nil
}

// ValidateRedisNode validates Redis cluster node format
func ValidateRedisNode(node string) error {
	if node == "" {
		return fmt.Errorf("Redis node cannot be empty")
	}

	// Check format: host:port
	host, port, err := net.SplitHostPort(node)
	if err != nil {
		return fmt.Errorf("Redis node must be in format host:port: %w", err)
	}

	if host == "" {
		return fmt.Errorf("Redis node must include host")
	}

	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("invalid Redis port: %w", err)
	}

	return nil
}

// ValidateWebhookURL validates webhook URLs for integrations
func ValidateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return nil // Empty is allowed (disabled)
	}

	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("webhook URL must use HTTPS")
	}

	return nil
}
