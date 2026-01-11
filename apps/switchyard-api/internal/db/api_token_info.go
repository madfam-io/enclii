package db

import (
	"github.com/google/uuid"
)

// APITokenInfo contains minimal token info needed for authentication
// This is used by the auth package to avoid circular dependencies
type APITokenInfo struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Name   string
	Scopes []string
}
