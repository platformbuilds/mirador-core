package models

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Validation errors
var (
	ErrEmptyID                      = errors.New("ID cannot be empty")
	ErrEmptyName                    = errors.New("name cannot be empty")
	ErrInvalidKind                  = errors.New("kind must be 'business' or 'tech'")
	ErrInvalidVisibility            = errors.New("visibility must be 'private', 'team', or 'org'")
	ErrInvalidSentiment             = errors.New("sentiment must be 'NEGATIVE', 'POSITIVE', or 'NEUTRAL'")
	ErrEmptyOwnerUserID             = errors.New("ownerUserId cannot be empty")
	ErrInvalidThresholdOperator     = errors.New("threshold operator must be 'gt', 'lt', 'eq', 'gte', 'lte'")
	ErrThresholdsNotOrdered         = errors.New("thresholds must be ordered by value (ascending for positive sentiment, descending for negative)")
	ErrInvalidGridCoordinates       = errors.New("grid coordinates must be non-negative")
	ErrInvalidGridDimensions        = errors.New("grid dimensions must be positive and fit within 12-column system")
	ErrCannotDeleteDefaultDashboard = errors.New("cannot delete the default dashboard")
	ErrInvalidVisibilityHierarchy   = errors.New("visibility hierarchy violation")
)

// ValidateKPIDefinition validates a KPI definition
func (k *KPIDefinition) Validate() error {
	if strings.TrimSpace(k.ID) == "" {
		return ErrEmptyID
	}

	if strings.TrimSpace(k.Name) == "" {
		return ErrEmptyName
	}

	if k.Kind != "business" && k.Kind != "tech" {
		return ErrInvalidKind
	}

	if k.Visibility != "private" && k.Visibility != "team" && k.Visibility != "org" {
		return ErrInvalidVisibility
	}

	if k.Sentiment != "" && k.Sentiment != "NEGATIVE" && k.Sentiment != "POSITIVE" && k.Sentiment != "NEUTRAL" {
		return ErrInvalidSentiment
	}

	if strings.TrimSpace(k.OwnerUserID) == "" {
		return ErrEmptyOwnerUserID
	}

	// Validate query field (must be present for formula-based KPIs)
	if len(k.Query) == 0 {
		return errors.New("query definition is required")
	}

	// Validate thresholds ordering
	if err := k.validateThresholds(); err != nil {
		return err
	}

	// Validate formula expressions if present
	if err := k.validateFormulaExpressions(); err != nil {
		return err
	}

	return nil
}

// validateThresholds validates threshold ordering and operators
func (k *KPIDefinition) validateThresholds() error {
	if len(k.Thresholds) == 0 {
		return nil // Thresholds are optional
	}

	for i, threshold := range k.Thresholds {
		if threshold.Level == "" {
			return fmt.Errorf("threshold %d: level cannot be empty", i)
		}

		if threshold.Operator == "" {
			return ErrInvalidThresholdOperator
		}

		validOperators := []string{"gt", "lt", "eq", "gte", "lte"}
		valid := false
		for _, op := range validOperators {
			if threshold.Operator == op {
				valid = true
				break
			}
		}
		if !valid {
			return ErrInvalidThresholdOperator
		}
	}

	// Check ordering based on sentiment
	if len(k.Thresholds) > 1 {
		switch k.Sentiment {
		case "POSITIVE":
			// For positive sentiment, thresholds should be in descending order (critical before warning)
			// e.g., revenue: critical < 500, warning < 1000
			for i := 1; i < len(k.Thresholds); i++ {
				if k.Thresholds[i].Value >= k.Thresholds[i-1].Value {
					return ErrThresholdsNotOrdered
				}
			}
		case "NEGATIVE":
			// For negative sentiment, thresholds should be in ascending order (warning before critical)
			// e.g., error rate: warning > 5%, critical > 10%
			for i := 1; i < len(k.Thresholds); i++ {
				if k.Thresholds[i].Value <= k.Thresholds[i-1].Value {
					return ErrThresholdsNotOrdered
				}
			}
		}
		// For NEUTRAL sentiment, no ordering requirement
	}

	return nil
}

// validateFormulaExpressions validates formula expressions in query
func (k *KPIDefinition) validateFormulaExpressions() error {
	// Look for formula expressions in the query map
	if formula, ok := k.Query["formula"].(string); ok && formula != "" {
		// Basic regex validation for formula expressions
		// Allow alphanumeric, operators (+, -, *, /, (), spaces), and metric references
		validFormulaRegex := regexp.MustCompile(`^[a-zA-Z0-9_\+\-\*\/\(\)\s\.]+$`)
		if !validFormulaRegex.MatchString(formula) {
			return errors.New("invalid characters in formula expression")
		}

		// Check for balanced parentheses
		if !hasBalancedParentheses(formula) {
			return errors.New("unbalanced parentheses in formula expression")
		}

		// Check for division by zero patterns (basic check)
		if strings.Contains(formula, "/0") || strings.Contains(formula, "/ 0") {
			return errors.New("division by zero detected in formula")
		}
	}

	return nil
}

// hasBalancedParentheses checks if parentheses are balanced in a string
func hasBalancedParentheses(s string) bool {
	count := 0
	for _, char := range s {
		switch char {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

// ValidateDashboard validates a dashboard
func (d *Dashboard) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return ErrEmptyID
	}

	if strings.TrimSpace(d.Name) == "" {
		return ErrEmptyName
	}

	if d.Visibility != "private" && d.Visibility != "team" && d.Visibility != "org" {
		return ErrInvalidVisibility
	}

	if strings.TrimSpace(d.OwnerUserID) == "" {
		return ErrEmptyOwnerUserID
	}

	return nil
}

// ValidateDashboardDeletion validates if a dashboard can be deleted
func (d *Dashboard) ValidateDeletion() error {
	if d.IsDefault {
		return ErrCannotDeleteDefaultDashboard
	}
	return nil
}

// ValidateKPILayout validates a KPI layout
func (l *KPILayout) Validate() error {
	if strings.TrimSpace(l.ID) == "" {
		return ErrEmptyID
	}

	if strings.TrimSpace(l.KPIDefinitionID) == "" {
		return errors.New("kpiDefinitionId cannot be empty")
	}

	if strings.TrimSpace(l.DashboardID) == "" {
		return errors.New("dashboardId cannot be empty")
	}

	// Validate grid coordinates
	if l.X < 0 || l.Y < 0 {
		return ErrInvalidGridCoordinates
	}

	// Validate dimensions
	if l.W <= 0 || l.H <= 0 {
		return ErrInvalidGridDimensions
	}

	// Check if layout fits within 12-column grid system
	if l.X+l.W > 12 {
		return ErrInvalidGridDimensions
	}

	// Reasonable bounds for grid dimensions
	if l.W > 12 || l.H > 20 {
		return ErrInvalidGridDimensions
	}

	return nil
}

// ValidateVisibilityHierarchy validates visibility hierarchy rules
// private < team < org (private can only see private, team can see private+team, org can see all)
func ValidateVisibilityHierarchy(requestorVisibility, resourceVisibility string) error {
	validVisibilities := []string{"private", "team", "org"}

	// Check if visibilities are valid
	if !contains(validVisibilities, requestorVisibility) || !contains(validVisibilities, resourceVisibility) {
		return ErrInvalidVisibility
	}

	// Define hierarchy levels (higher number = more permissive)
	hierarchy := map[string]int{
		"private": 1,
		"team":    2,
		"org":     3,
	}

	requestorLevel := hierarchy[requestorVisibility]
	resourceLevel := hierarchy[resourceVisibility]

	// Requestor must have equal or higher visibility level than the resource
	if requestorLevel < resourceLevel {
		return ErrInvalidVisibilityHierarchy
	}

	return nil
}

// CanAccessResource checks if a user with given visibility can access a resource with given visibility
func CanAccessResource(userVisibility, resourceVisibility string) bool {
	return ValidateVisibilityHierarchy(userVisibility, resourceVisibility) == nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
