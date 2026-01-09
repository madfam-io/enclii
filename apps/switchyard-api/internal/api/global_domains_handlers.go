package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// DomainWithContext extends CustomDomain with service and environment context
type DomainWithContext struct {
	types.CustomDomain
	ServiceName     string `json:"service_name"`
	EnvironmentName string `json:"environment_name"`
	ProjectSlug     string `json:"project_slug,omitempty"`
}

// DomainsListResponse represents the paginated domains response
type DomainsListResponse struct {
	Domains []DomainWithContext `json:"domains"`
	Total   int                 `json:"total"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
}

// GetAllDomains returns all custom domains across all services
// GET /v1/domains
func (h *Handler) GetAllDomains(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Build filters
	filters := make(map[string]interface{})

	if verified := c.Query("verified"); verified != "" {
		if verified == "true" {
			filters["verified"] = true
		} else if verified == "false" {
			filters["verified"] = false
		}
	}

	if tlsEnabled := c.Query("tls_enabled"); tlsEnabled != "" {
		if tlsEnabled == "true" {
			filters["tls_enabled"] = true
		} else if tlsEnabled == "false" {
			filters["tls_enabled"] = false
		}
	}

	// Query domains
	domains, total, err := h.repos.CustomDomains.ListAll(ctx, filters, limit, offset)
	if err != nil {
		h.logger.Error(ctx, "Failed to list domains", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domains"})
		return
	}

	// Enrich domains with service and environment context
	enrichedDomains := make([]DomainWithContext, 0, len(domains))
	for _, domain := range domains {
		enriched := DomainWithContext{
			CustomDomain: domain,
		}

		// Get service info
		if service, err := h.repos.Services.GetByID(domain.ServiceID); err == nil && service != nil {
			enriched.ServiceName = service.Name

			// Get project slug for navigation
			if project, err := h.repos.Projects.GetByID(ctx, service.ProjectID); err == nil && project != nil {
				enriched.ProjectSlug = project.Slug
			}
		}

		// Get environment info
		if env, err := h.repos.Environments.GetByID(ctx, domain.EnvironmentID); err == nil && env != nil {
			enriched.EnvironmentName = env.Name
		}

		enrichedDomains = append(enrichedDomains, enriched)
	}

	c.JSON(http.StatusOK, DomainsListResponse{
		Domains: enrichedDomains,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

// GetDomainStats returns statistics about domains
// GET /v1/domains/stats
func (h *Handler) GetDomainStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Get all domains to calculate stats
	filters := make(map[string]interface{})
	domains, _, err := h.repos.CustomDomains.ListAll(ctx, filters, 1000, 0)
	if err != nil {
		h.logger.Error(ctx, "Failed to get domain stats", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domain stats"})
		return
	}

	// Calculate statistics
	var totalDomains, verifiedDomains, pendingDomains, tlsEnabled int
	var platformDomains, customDomains int

	for _, domain := range domains {
		totalDomains++
		if domain.Verified {
			verifiedDomains++
		} else {
			pendingDomains++
		}
		if domain.TLSEnabled {
			tlsEnabled++
		}
		if domain.IsPlatformDomain {
			platformDomains++
		} else {
			customDomains++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_domains":    totalDomains,
		"verified_domains": verifiedDomains,
		"pending_domains":  pendingDomains,
		"tls_enabled":      tlsEnabled,
		"platform_domains": platformDomains,
		"custom_domains":   customDomains,
	})
}
