package auth

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetUserIDFromContext extracts the user ID from the Gin context
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, fmt.Errorf("user ID not found in context")
	}

	// Handle both string and uuid.UUID types for backwards compatibility
	switch v := userID.(type) {
	case uuid.UUID:
		return v, nil
	case string:
		return uuid.Parse(v)
	default:
		return uuid.Nil, fmt.Errorf("invalid user ID format: expected string or uuid.UUID, got %T", userID)
	}
}

// GetUserEmailFromContext extracts the user email from the Gin context
func GetUserEmailFromContext(c *gin.Context) (string, error) {
	email, exists := c.Get("user_email")
	if !exists {
		return "", fmt.Errorf("user email not found in context")
	}

	emailStr, ok := email.(string)
	if !ok {
		return "", fmt.Errorf("invalid email format")
	}

	return emailStr, nil
}

// GetClaimsFromContext extracts the JWT claims from the Gin context
func GetClaimsFromContext(c *gin.Context) (*Claims, error) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, fmt.Errorf("claims not found in context")
	}

	claimsObj, ok := claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	return claimsObj, nil
}
