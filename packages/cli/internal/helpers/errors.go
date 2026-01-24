// Package helpers provides common utilities for CLI commands.
package helpers

import (
	"fmt"
)

// Action represents common CLI operation verbs for error messages.
type Action string

const (
	ActionParse    Action = "parse"
	ActionFind     Action = "find"
	ActionGet      Action = "get"
	ActionList     Action = "list"
	ActionCreate   Action = "create"
	ActionUpdate   Action = "update"
	ActionDelete   Action = "delete"
	ActionBuild    Action = "build"
	ActionDeploy   Action = "deploy"
	ActionRollback Action = "rollback"
	ActionVerify   Action = "verify"
	ActionSet      Action = "set"
	ActionReveal   Action = "reveal"
	ActionEnsure   Action = "ensure"
)

// WrapError creates a consistently formatted error with the pattern "failed to <action> <resource>: <err>".
// This consolidates the 67+ duplicate fmt.Errorf patterns across CLI commands.
//
// Example:
//
//	WrapError(ActionParse, "service.yaml", err)
//	// Returns: "failed to parse service.yaml: <original error>"
func WrapError(action Action, resource string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to %s %s: %w", action, resource, err)
}

// WrapErrorf creates a consistently formatted error with additional context.
// Use when you need to include dynamic values in the resource description.
//
// Example:
//
//	WrapErrorf(ActionFind, err, "environment %s in project %s", envName, projectSlug)
//	// Returns: "failed to find environment staging in project myproject: <original error>"
func WrapErrorf(action Action, err error, resourceFmt string, args ...any) error {
	if err == nil {
		return nil
	}
	resource := fmt.Sprintf(resourceFmt, args...)
	return fmt.Errorf("failed to %s %s: %w", action, resource, err)
}

// NewNotFoundError creates a standardized "not found" error.
//
// Example:
//
//	NewNotFoundError("service", "my-api", "project", "default")
//	// Returns: "service my-api not found in project default"
func NewNotFoundError(resourceType, resourceName, scopeType, scopeName string) error {
	return fmt.Errorf("%s %s not found in %s %s", resourceType, resourceName, scopeType, scopeName)
}

// NewValidationError creates a standardized validation error.
//
// Example:
//
//	NewValidationError("KEY=VALUE", "invalid format, expected KEY=VALUE")
//	// Returns: "invalid value \"KEY=VALUE\": invalid format, expected KEY=VALUE"
func NewValidationError(value, reason string) error {
	return fmt.Errorf("invalid value %q: %s", value, reason)
}

// NewRequiredError creates a standardized "required" error.
//
// Example:
//
//	NewRequiredError("service name")
//	// Returns: "service name is required"
func NewRequiredError(field string) error {
	return fmt.Errorf("%s is required", field)
}

// NewTimeoutError creates a standardized timeout error.
//
// Example:
//
//	NewTimeoutError("build", "10 minutes")
//	// Returns: "build timeout after 10 minutes"
func NewTimeoutError(operation, duration string) error {
	return fmt.Errorf("%s timeout after %s", operation, duration)
}
