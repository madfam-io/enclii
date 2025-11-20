package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Page  int
	Limit int
	Sort  string
	Order string
}

// PaginationResponse is the standard pagination response
type PaginationResponse struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// DefaultPageSize is the default number of items per page
const DefaultPageSize = 20

// MaxPageSize is the maximum number of items per page
const MaxPageSize = 100

// GetPaginationParams extracts pagination parameters from the request
func GetPaginationParams(c *gin.Context) PaginationParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(DefaultPageSize)))
	sort := c.DefaultQuery("sort", "created_at")
	order := c.DefaultQuery("order", "desc")

	// Validate and sanitize parameters
	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = DefaultPageSize
	}

	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	// Validate order
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	return PaginationParams{
		Page:  page,
		Limit: limit,
		Sort:  sort,
		Order: order,
	}
}

// CalculateOffset calculates the SQL OFFSET value
func (p *PaginationParams) CalculateOffset() int {
	return (p.Page - 1) * p.Limit
}

// BuildPaginationResponse builds a pagination response from total count
func (p *PaginationParams) BuildPaginationResponse(total int64) PaginationResponse {
	totalPages := int((total + int64(p.Limit) - 1) / int64(p.Limit))

	return PaginationResponse{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    p.Page < totalPages,
		HasPrev:    p.Page > 1,
	}
}

// PaginatedData wraps data with pagination info
type PaginatedData struct {
	Data       interface{}         `json:"data"`
	Pagination PaginationResponse `json:"pagination"`
}

// NewPaginatedData creates a new paginated data response
func NewPaginatedData(data interface{}, params PaginationParams, total int64) PaginatedData {
	return PaginatedData{
		Data:       data,
		Pagination: params.BuildPaginationResponse(total),
	}
}
