/**
 * API utility for making authenticated requests to the Switchyard API
 *
 * SECURITY: Implements authentication with JWT tokens and CSRF protection.
 * For production deployment with OAuth 2.0 / OIDC, see:
 * - contexts/AuthContext.tsx
 * - SECURITY_AUDIT_COMPREHENSIVE_2025.md
 */

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:4200";

// CSRF token cache
let csrfToken: string | null = null;

/**
 * Get authentication headers for API requests
 *
 * Retrieves JWT token from localStorage (set by AuthContext)
 * Includes CSRF token for write operations
 */
function getAuthHeaders(includeCSRF: boolean = false): HeadersInit {
  const headers: HeadersInit = {
    "Content-Type": "application/json",
  };

  // Get JWT token from localStorage
  if (typeof window !== "undefined") {
    const token = localStorage.getItem("auth_token");
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    } else {
      // Development fallback
      const devToken = process.env.NEXT_PUBLIC_API_TOKEN;
      if (devToken) {
        headers["Authorization"] = `Bearer ${devToken}`;
      }
    }
  }

  // Add CSRF token for write operations
  if (includeCSRF && csrfToken) {
    headers["X-CSRF-Token"] = csrfToken;
  }

  return headers;
}

/**
 * Fetch and cache CSRF token
 */
async function fetchCSRFToken(): Promise<void> {
  try {
    const response = await fetch(`${API_BASE_URL}/v1/csrf`, {
      credentials: "include", // Include cookies
    });

    if (response.ok) {
      const token = response.headers.get("X-CSRF-Token");
      if (token) {
        csrfToken = token;
      }
    }
  } catch (error) {
    console.error("Failed to fetch CSRF token:", error);
  }
}

/**
 * Make an authenticated API request with CSRF protection
 *
 * @param endpoint - API endpoint path (e.g., '/v1/projects')
 * @param options - Fetch options (method, body, etc.)
 * @returns Promise with the response
 */
export async function apiRequest<T = any>(
  endpoint: string,
  options: RequestInit = {},
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;
  const method = options.method || "GET";
  const isWriteOperation = ["POST", "PUT", "DELETE", "PATCH"].includes(
    method.toUpperCase(),
  );

  // Fetch CSRF token for write operations if not cached
  if (isWriteOperation && !csrfToken) {
    await fetchCSRFToken();
  }

  const headers: HeadersInit = {
    ...getAuthHeaders(isWriteOperation),
    ...options.headers,
  };

  try {
    const response = await fetch(url, {
      ...options,
      headers,
      credentials: "include", // Include cookies for CSRF
    });

    // Handle authentication errors
    if (response.status === 401) {
      // Clear invalid token
      if (typeof window !== "undefined") {
        localStorage.removeItem("auth_token");
      }
      throw new Error("Authentication required. Please log in again.");
    }

    if (response.status === 403) {
      throw new Error(
        "Access denied. You do not have permission to perform this action.",
      );
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(
        error.message ||
          `API request failed: ${response.status} ${response.statusText}`,
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
  return apiRequest<T>(endpoint, { method: "GET" });
}

/**
 * POST request helper
 */
export async function apiPost<T = any>(
  endpoint: string,
  data: any,
): Promise<T> {
  return apiRequest<T>(endpoint, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

/**
 * PUT request helper
 */
export async function apiPut<T = any>(endpoint: string, data: any): Promise<T> {
  return apiRequest<T>(endpoint, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

/**
 * DELETE request helper
 */
export async function apiDelete<T = any>(endpoint: string): Promise<T> {
  return apiRequest<T>(endpoint, { method: "DELETE" });
}

/**
 * Pagination parameters
 */
export interface PaginationParams {
  page?: number;
  limit?: number;
  sort?: string;
  order?: "asc" | "desc";
}

/**
 * Paginated response
 */
export interface PaginatedResponse<T> {
  data: T[];
  pagination: {
    page: number;
    limit: number;
    total: number;
    totalPages: number;
    hasNext: boolean;
    hasPrev: boolean;
  };
}

/**
 * GET request with pagination support
 */
export async function apiGetPaginated<T = any>(
  endpoint: string,
  params?: PaginationParams,
): Promise<PaginatedResponse<T>> {
  const queryParams = new URLSearchParams();

  if (params?.page) queryParams.append("page", params.page.toString());
  if (params?.limit) queryParams.append("limit", params.limit.toString());
  if (params?.sort) queryParams.append("sort", params.sort);
  if (params?.order) queryParams.append("order", params.order);

  const url = queryParams.toString()
    ? `${endpoint}?${queryParams.toString()}`
    : endpoint;

  return apiRequest<PaginatedResponse<T>>(url, { method: "GET" });
}
