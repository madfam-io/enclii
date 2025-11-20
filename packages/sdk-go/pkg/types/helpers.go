package types

import (
	"github.com/google/uuid"
)

// UUID helper methods for domain types

// Service helpers
func (s *Service) IDString() string {
	return s.ID.String()
}

func (s *Service) ProjectIDString() string {
	return s.ProjectID.String()
}

// Project helpers
func (p *Project) IDString() string {
	return p.ID.String()
}

// Environment helpers
func (e *Environment) IDString() string {
	return e.ID.String()
}

func (e *Environment) ProjectIDString() string {
	return e.ProjectID.String()
}

// Release helpers
func (r *Release) IDString() string {
	return r.ID.String()
}

func (r *Release) ServiceIDString() string {
	return r.ServiceID.String()
}

// Deployment helpers
func (d *Deployment) IDString() string {
	return d.ID.String()
}

func (d *Deployment) ReleaseIDString() string {
	return d.ReleaseID.String()
}

func (d *Deployment) EnvironmentIDString() string {
	return d.EnvironmentID.String()
}

// User helpers
func (u *User) IDString() string {
	return u.ID.String()
}

// ParseUUID is a helper that wraps uuid.Parse with better error messaging
func ParseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// MustParseUUID parses a UUID string and panics on error (use only for constants)
func MustParseUUID(s string) uuid.UUID {
	return uuid.MustParse(s)
}

// NewUUID generates a new UUID
func NewUUID() uuid.UUID {
	return uuid.New()
}

// IsValidUUID checks if a string is a valid UUID
func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
