package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
)

// UsageMetric represents a single usage metric
type UsageMetric struct {
	Type     string  `json:"type"`
	Label    string  `json:"label"`
	Used     float64 `json:"used"`
	Included float64 `json:"included"` // -1 for unlimited
	Unit     string  `json:"unit"`
	Cost     float64 `json:"cost"`
}

// UsageSummary represents the complete usage data for billing
type UsageSummary struct {
	PeriodStart string        `json:"period_start"`
	PeriodEnd   string        `json:"period_end"`
	Metrics     []UsageMetric `json:"metrics"`
	TotalCost   float64       `json:"total_cost"`
	PlanBase    float64       `json:"plan_base"`
	GrandTotal  float64       `json:"grand_total"`
	PlanName    string        `json:"plan_name"`
}

// CostCategory represents a cost breakdown category
type CostCategory struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Color string  `json:"color"`
}

// CostBreakdown represents the billing cost breakdown
type CostBreakdown struct {
	PeriodStart string         `json:"period_start"`
	PeriodEnd   string         `json:"period_end"`
	PlanBase    float64        `json:"plan_base"`
	PlanName    string         `json:"plan_name"`
	Categories  []CostCategory `json:"categories"`
	TotalUsage  float64        `json:"total_usage"`
	GrandTotal  float64        `json:"grand_total"`
}

// Plan pricing constants (in a real system, these would come from a billing service)
const (
	proPlanBase      = 20.00
	computePerGBHour = 0.05
	buildPerMinute   = 0.01
	storagePerGB     = 0.25
	bandwidthPerGB   = 0.10
)

// Included resources per plan
const (
	includedCompute   = 500.0 // GB-hours
	includedBuild     = 500.0 // minutes
	includedStorage   = 10.0  // GB
	includedBandwidth = 500.0 // GB
)

// GetUsageSummary returns the current usage metrics for billing
// GET /v1/usage
func (h *Handler) GetUsageSummary(c *gin.Context) {
	ctx := c.Request.Context()

	// Get current billing period (1st of month to end of month)
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

	// Calculate usage from actual data
	usage, err := h.calculateUsage(ctx, periodStart, periodEnd)
	if err != nil {
		h.logger.Error(ctx, "Failed to calculate usage", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate usage"})
		return
	}

	c.JSON(http.StatusOK, usage)
}

// GetCostBreakdown returns the cost breakdown for billing
// GET /v1/usage/costs
func (h *Handler) GetCostBreakdown(c *gin.Context) {
	ctx := c.Request.Context()

	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

	breakdown, err := h.calculateCostBreakdown(ctx, periodStart, periodEnd)
	if err != nil {
		h.logger.Error(ctx, "Failed to calculate costs", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate costs"})
		return
	}

	c.JSON(http.StatusOK, breakdown)
}

// calculateUsage computes usage metrics from actual data
func (h *Handler) calculateUsage(ctx context.Context, periodStart, periodEnd time.Time) (*UsageSummary, error) {
	// Count services for compute estimation
	services, err := h.repos.Services.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// Count releases for build minutes estimation
	var totalBuilds int
	for _, svc := range services {
		releases, err := h.repos.Releases.ListByService(svc.ID)
		if err != nil {
			continue
		}
		for _, rel := range releases {
			if rel.CreatedAt.After(periodStart) && rel.CreatedAt.Before(periodEnd) {
				totalBuilds++
			}
		}
	}

	// Count custom domains
	var totalDomains int
	for _, svc := range services {
		domains, err := h.repos.CustomDomains.GetByServiceID(ctx, svc.ID.String())
		if err != nil {
			continue
		}
		totalDomains += len(domains)
	}

	// Calculate metrics based on actual usage
	// Estimate: each running service uses ~5 GB-hours per day on average
	daysInPeriod := time.Since(periodStart).Hours() / 24
	if daysInPeriod < 1 {
		daysInPeriod = 1
	}
	computeUsed := float64(len(services)) * 5.0 * daysInPeriod

	// Estimate: each build takes ~3 minutes on average
	buildMinutes := float64(totalBuilds) * 3.0

	// Estimate storage: ~0.5 GB per service for images and artifacts
	storageUsed := float64(len(services)) * 0.5

	// Estimate bandwidth: ~10 GB per service per billing period
	bandwidthUsed := float64(len(services)) * 10.0 * (daysInPeriod / 30.0)

	// Calculate overage costs
	computeCost := calculateOverage(computeUsed, includedCompute, computePerGBHour)
	buildCost := calculateOverage(buildMinutes, includedBuild, buildPerMinute)
	storageCost := calculateOverage(storageUsed, includedStorage, storagePerGB)
	bandwidthCost := calculateOverage(bandwidthUsed, includedBandwidth, bandwidthPerGB)

	totalCost := computeCost + buildCost + storageCost + bandwidthCost

	metrics := []UsageMetric{
		{
			Type:     "compute",
			Label:    "Compute",
			Used:     roundToTwoDecimals(computeUsed),
			Included: includedCompute,
			Unit:     "GB-hours",
			Cost:     roundToTwoDecimals(computeCost),
		},
		{
			Type:     "build",
			Label:    "Build Minutes",
			Used:     roundToTwoDecimals(buildMinutes),
			Included: includedBuild,
			Unit:     "minutes",
			Cost:     roundToTwoDecimals(buildCost),
		},
		{
			Type:     "storage",
			Label:    "Storage",
			Used:     roundToTwoDecimals(storageUsed),
			Included: includedStorage,
			Unit:     "GB",
			Cost:     roundToTwoDecimals(storageCost),
		},
		{
			Type:     "bandwidth",
			Label:    "Bandwidth",
			Used:     roundToTwoDecimals(bandwidthUsed),
			Included: includedBandwidth,
			Unit:     "GB",
			Cost:     roundToTwoDecimals(bandwidthCost),
		},
		{
			Type:     "domains",
			Label:    "Custom Domains",
			Used:     float64(totalDomains),
			Included: -1, // Unlimited
			Unit:     "domains",
			Cost:     0,
		},
	}

	return &UsageSummary{
		PeriodStart: periodStart.Format("2006-01-02"),
		PeriodEnd:   periodEnd.Format("2006-01-02"),
		Metrics:     metrics,
		TotalCost:   roundToTwoDecimals(totalCost),
		PlanBase:    proPlanBase,
		GrandTotal:  roundToTwoDecimals(proPlanBase + totalCost),
		PlanName:    "Pro",
	}, nil
}

// calculateCostBreakdown computes cost breakdown from actual data
func (h *Handler) calculateCostBreakdown(ctx context.Context, periodStart, periodEnd time.Time) (*CostBreakdown, error) {
	usage, err := h.calculateUsage(ctx, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	categories := []CostCategory{
		{Name: "Compute", Value: usage.Metrics[0].Cost, Color: "#3b82f6"},
		{Name: "Build", Value: usage.Metrics[1].Cost, Color: "#22c55e"},
		{Name: "Storage", Value: usage.Metrics[2].Cost, Color: "#f59e0b"},
		{Name: "Bandwidth", Value: usage.Metrics[3].Cost, Color: "#8b5cf6"},
	}

	return &CostBreakdown{
		PeriodStart: usage.PeriodStart,
		PeriodEnd:   usage.PeriodEnd,
		PlanBase:    proPlanBase,
		PlanName:    "Pro",
		Categories:  categories,
		TotalUsage:  usage.TotalCost,
		GrandTotal:  usage.GrandTotal,
	}, nil
}

// calculateOverage calculates overage cost
func calculateOverage(used, included, pricePerUnit float64) float64 {
	if used <= included {
		return 0
	}
	return (used - included) * pricePerUnit
}

// roundToTwoDecimals rounds a float to 2 decimal places
func roundToTwoDecimals(val float64) float64 {
	return float64(int(val*100)) / 100
}
