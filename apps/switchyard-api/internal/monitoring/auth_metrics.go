package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Authentication Prometheus metrics
var (
	// Counter: Total authentication requests
	authRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "enclii_auth_requests_total",
			Help: "Total number of authentication requests",
		},
		[]string{"method", "status"}, // method: login|refresh|logout, status: success|failure
	)

	// Counter: Token validations
	authTokenValidationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "enclii_auth_token_validations_total",
			Help: "Total number of token validation attempts",
		},
		[]string{"source", "status"}, // source: local|external, status: valid|invalid
	)

	// Histogram: Auth request duration
	authRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "enclii_auth_request_duration_seconds",
			Help:    "Authentication request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"method"}, // method: login|refresh|logout|validate
	)

	// Histogram: JWKS fetch duration
	jwksFetchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "enclii_jwks_fetch_duration_seconds",
			Help:    "JWKS fetch duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"provider"}, // provider: janua|local|etc
	)

	// Gauge: Active sessions
	activeSessionsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "enclii_active_sessions_total",
			Help: "Number of active authenticated sessions",
		},
	)

	// Gauge: JWKS cache age
	jwksCacheAgeSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "enclii_jwks_cache_age_seconds",
			Help: "Age of the JWKS cache in seconds",
		},
	)

	// Counter: JWKS fetch failures
	jwksFetchFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "enclii_jwks_fetch_failures_total",
			Help: "Total number of JWKS fetch failures",
		},
		[]string{"provider", "error_type"}, // error_type: network|parse|timeout
	)

	// Counter: Rate limit hits
	rateLimitHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "enclii_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"endpoint", "key_type"}, // key_type: ip|user
	)

	// Counter: RBAC denials
	rbacDenialsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "enclii_rbac_denials_total",
			Help: "Total number of RBAC permission denials",
		},
		[]string{"permission", "role"},
	)

	// Counter: Session revocations
	sessionRevocationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "enclii_session_revocations_total",
			Help: "Total number of session revocations",
		},
		[]string{"reason"}, // reason: logout|refresh|admin|security
	)

	// Gauge: JWKS cache key count
	jwksCacheKeyCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "enclii_jwks_cache_key_count",
			Help: "Number of keys in JWKS cache",
		},
	)

	// Counter: External user creations
	externalUserCreationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "enclii_external_user_creations_total",
			Help: "Total number of users created from external tokens",
		},
		[]string{"issuer"},
	)
)

// =============================================================================
// Metric recording functions for auth package
// =============================================================================

// RecordAuthRequest records an authentication request
func RecordAuthRequest(method, status string) {
	authRequestsTotal.WithLabelValues(method, status).Inc()
}

// RecordTokenValidation records a token validation attempt
func RecordTokenValidation(source, status string) {
	authTokenValidationsTotal.WithLabelValues(source, status).Inc()
}

// RecordAuthDuration records the duration of an auth operation
func RecordAuthDuration(method string, durationSeconds float64) {
	authRequestDuration.WithLabelValues(method).Observe(durationSeconds)
}

// RecordJWKSFetch records a JWKS fetch operation
func RecordJWKSFetch(provider string, durationSeconds float64) {
	jwksFetchDuration.WithLabelValues(provider).Observe(durationSeconds)
}

// RecordJWKSFetchFailure records a JWKS fetch failure
func RecordJWKSFetchFailure(provider, errorType string) {
	jwksFetchFailuresTotal.WithLabelValues(provider, errorType).Inc()
}

// SetActiveSessions sets the active session count
func SetActiveSessions(count int) {
	activeSessionsTotal.Set(float64(count))
}

// IncrementActiveSessions increments active sessions by 1
func IncrementActiveSessions() {
	activeSessionsTotal.Inc()
}

// DecrementActiveSessions decrements active sessions by 1
func DecrementActiveSessions() {
	activeSessionsTotal.Dec()
}

// SetJWKSCacheAge sets the JWKS cache age in seconds
func SetJWKSCacheAge(ageSeconds float64) {
	jwksCacheAgeSeconds.Set(ageSeconds)
}

// SetJWKSCacheKeyCount sets the number of keys in JWKS cache
func SetJWKSCacheKeyCount(count int) {
	jwksCacheKeyCount.Set(float64(count))
}

// RecordRateLimitHit records when a rate limit is hit
func RecordRateLimitHit(endpoint, keyType string) {
	rateLimitHitsTotal.WithLabelValues(endpoint, keyType).Inc()
}

// RecordRBACDenial records when an RBAC permission check fails
func RecordRBACDenial(permission, role string) {
	rbacDenialsTotal.WithLabelValues(permission, role).Inc()
}

// RecordSessionRevocation records a session revocation
func RecordSessionRevocation(reason string) {
	sessionRevocationsTotal.WithLabelValues(reason).Inc()
}

// RecordExternalUserCreation records when a user is created from external token
func RecordExternalUserCreation(issuer string) {
	externalUserCreationsTotal.WithLabelValues(issuer).Inc()
}

// =============================================================================
// Convenience functions for common auth scenarios
// =============================================================================

// RecordLoginSuccess records a successful login
func RecordLoginSuccess(method string, durationSeconds float64) {
	RecordAuthRequest("login", "success")
	RecordAuthDuration("login", durationSeconds)
	IncrementActiveSessions()
}

// RecordLoginFailure records a failed login
func RecordLoginFailure(method string, durationSeconds float64) {
	RecordAuthRequest("login", "failure")
	RecordAuthDuration("login", durationSeconds)
}

// RecordLogout records a logout
func RecordLogout(durationSeconds float64) {
	RecordAuthRequest("logout", "success")
	RecordAuthDuration("logout", durationSeconds)
	DecrementActiveSessions()
	RecordSessionRevocation("logout")
}

// RecordTokenRefresh records a token refresh
func RecordTokenRefresh(success bool, durationSeconds float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	RecordAuthRequest("refresh", status)
	RecordAuthDuration("refresh", durationSeconds)
	if success {
		RecordSessionRevocation("refresh")
	}
}

// RecordLocalTokenValidation records a local token validation
func RecordLocalTokenValidation(valid bool) {
	status := "valid"
	if !valid {
		status = "invalid"
	}
	RecordTokenValidation("local", status)
}

// RecordExternalTokenValidation records an external token validation
func RecordExternalTokenValidation(valid bool) {
	status := "valid"
	if !valid {
		status = "invalid"
	}
	RecordTokenValidation("external", status)
}
