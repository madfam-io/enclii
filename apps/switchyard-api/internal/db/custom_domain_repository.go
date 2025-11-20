package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// CustomDomainRepository handles database operations for custom domains
type CustomDomainRepository struct {
	db *sql.DB
}

func NewCustomDomainRepository(db *sql.DB) *CustomDomainRepository {
	return &CustomDomainRepository{db: db}
}

// Create adds a new custom domain
func (r *CustomDomainRepository) Create(ctx context.Context, domain *types.CustomDomain) error {
	query := `
		INSERT INTO custom_domains (
			id, service_id, environment_id, domain, verified, tls_enabled, tls_issuer,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	domain.ID = uuid.New()

	err := r.db.QueryRowContext(
		ctx,
		query,
		domain.ID,
		domain.ServiceID,
		domain.EnvironmentID,
		domain.Domain,
		domain.Verified,
		domain.TLSEnabled,
		domain.TLSIssuer,
	).Scan(&domain.ID, &domain.CreatedAt, &domain.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create custom domain: %w", err)
	}

	return nil
}

// GetByID retrieves a custom domain by ID
func (r *CustomDomainRepository) GetByID(ctx context.Context, id string) (*types.CustomDomain, error) {
	query := `
		SELECT id, service_id, environment_id, domain, verified, tls_enabled, tls_issuer,
		       created_at, updated_at, verified_at
		FROM custom_domains
		WHERE id = $1
	`

	domain := &types.CustomDomain{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&domain.ID,
		&domain.ServiceID,
		&domain.EnvironmentID,
		&domain.Domain,
		&domain.Verified,
		&domain.TLSEnabled,
		&domain.TLSIssuer,
		&domain.CreatedAt,
		&domain.UpdatedAt,
		&domain.VerifiedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("custom domain not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get custom domain: %w", err)
	}

	return domain, nil
}

// GetByServiceID retrieves all custom domains for a service
func (r *CustomDomainRepository) GetByServiceID(ctx context.Context, serviceID string) ([]types.CustomDomain, error) {
	query := `
		SELECT id, service_id, environment_id, domain, verified, tls_enabled, tls_issuer,
		       created_at, updated_at, verified_at
		FROM custom_domains
		WHERE service_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query custom domains: %w", err)
	}
	defer rows.Close()

	var domains []types.CustomDomain
	for rows.Next() {
		var domain types.CustomDomain
		err := rows.Scan(
			&domain.ID,
			&domain.ServiceID,
			&domain.EnvironmentID,
			&domain.Domain,
			&domain.Verified,
			&domain.TLSEnabled,
			&domain.TLSIssuer,
			&domain.CreatedAt,
			&domain.UpdatedAt,
			&domain.VerifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan custom domain: %w", err)
		}
		domains = append(domains, domain)
	}

	return domains, nil
}

// GetByServiceAndEnvironment retrieves custom domains for a service in a specific environment
func (r *CustomDomainRepository) GetByServiceAndEnvironment(ctx context.Context, serviceID, environmentID string) ([]types.CustomDomain, error) {
	query := `
		SELECT id, service_id, environment_id, domain, verified, tls_enabled, tls_issuer,
		       created_at, updated_at, verified_at
		FROM custom_domains
		WHERE service_id = $1 AND environment_id = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, serviceID, environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query custom domains: %w", err)
	}
	defer rows.Close()

	var domains []types.CustomDomain
	for rows.Next() {
		var domain types.CustomDomain
		err := rows.Scan(
			&domain.ID,
			&domain.ServiceID,
			&domain.EnvironmentID,
			&domain.Domain,
			&domain.Verified,
			&domain.TLSEnabled,
			&domain.TLSIssuer,
			&domain.CreatedAt,
			&domain.UpdatedAt,
			&domain.VerifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan custom domain: %w", err)
		}
		domains = append(domains, domain)
	}

	return domains, nil
}

// Update updates a custom domain
func (r *CustomDomainRepository) Update(ctx context.Context, domain *types.CustomDomain) error {
	query := `
		UPDATE custom_domains
		SET verified = $1, tls_enabled = $2, tls_issuer = $3, verified_at = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		domain.Verified,
		domain.TLSEnabled,
		domain.TLSIssuer,
		domain.VerifiedAt,
		domain.ID,
	).Scan(&domain.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update custom domain: %w", err)
	}

	return nil
}

// Delete removes a custom domain
func (r *CustomDomainRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM custom_domains WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete custom domain: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("custom domain not found: %s", id)
	}

	return nil
}

// Exists checks if a domain is already registered
func (r *CustomDomainRepository) Exists(ctx context.Context, domain string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM custom_domains WHERE domain = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, domain).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check domain existence: %w", err)
	}

	return exists, nil
}
