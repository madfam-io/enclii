/**
 * API utility for making authenticated requests to the Switchyard API
 *
 * SECURITY WARNING: This file currently does NOT implement authentication.
 * Before deploying to production, you MUST implement proper authentication:
 * - OAuth 2.0 / OIDC (recommended for production)
 * - Session-based auth with secure cookies
 * - Token-based auth with secure storage
 *
 * Current implementation is for development/testing ONLY.
 */

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

/**
 * Get authentication headers for API requests
 *
 * TODO: Replace with actual authentication implementation
 * This should retrieve a valid token from:
 * - Secure HTTP-only cookies
 * - OAuth provider (e.g., Auth0, Okta)
 * - Session storage (with proper security)
 */
function getAuthHeaders(): HeadersInit {
  // SECURITY: In production, implement proper auth token retrieval
  // For now, attempt to read from environment (development only)
  const token = process.env.NEXT_PUBLIC_API_TOKEN;

  if (!token) {
    console.warn(
      'WARNING: No API authentication token configured. ' +
      'API requests will fail without proper authentication. ' +
      'Set NEXT_PUBLIC_API_TOKEN environment variable for development, ' +
      'or implement OAuth 2.0 / OIDC for production.'
    );
    return {};
  }

  return {
    'Authorization': `Bearer ${token}`,
  };
}

/**
 * Make an authenticated API request
 *
 * @param endpoint - API endpoint path (e.g., '/api/v1/projects')
 * @param options - Fetch options (method, body, etc.)
 * @returns Promise with the response
 */
export async function apiRequest<T = any>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...getAuthHeaders(),
    ...options.headers,
  };

  try {
    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(
        error.message || `API request failed: ${response.status} ${response.statusText}`
      );
    }

    return await response.json();
  } catch (error) {
    console.error(`API request failed for ${endpoint}:`, error);
    throw error;
  }
}

/**
 * GET request helper
 */
export async function apiGet<T = any>(endpoint: string): Promise<T> {
  return apiRequest<T>(endpoint, { method: 'GET' });
}

/**
 * POST request helper
 */
export async function apiPost<T = any>(
  endpoint: string,
  data: any
): Promise<T> {
  return apiRequest<T>(endpoint, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * PUT request helper
 */
export async function apiPut<T = any>(
  endpoint: string,
  data: any
): Promise<T> {
  return apiRequest<T>(endpoint, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * DELETE request helper
 */
export async function apiDelete<T = any>(endpoint: string): Promise<T> {
  return apiRequest<T>(endpoint, { method: 'DELETE' });
}
