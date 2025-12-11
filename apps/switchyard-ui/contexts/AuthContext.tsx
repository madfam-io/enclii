"use client";

import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  ReactNode,
} from "react";

/**
 * Authentication Context for Enclii Switchyard UI
 *
 * Supports both local authentication and OIDC/OAuth via Janua/Janua.
 *
 * Authentication Modes:
 * - Local: Email/password directly to Switchyard API
 * - OIDC: OAuth 2.0 flow via external identity provider (Janua)
 *
 * The auth mode is determined by NEXT_PUBLIC_AUTH_MODE environment variable.
 */

// =============================================================================
// TYPES
// =============================================================================

interface User {
  id: string;
  email: string;
  name?: string;
  roles?: string[];
  avatarUrl?: string;
}

interface TokenInfo {
  accessToken: string;
  refreshToken?: string;
  expiresAt: number; // Unix timestamp
  tokenType: string;
  // IDP token from identity provider (e.g., Janua) for calling IDP-specific APIs
  idpToken?: string;
  idpTokenExpiresAt?: number; // Unix timestamp
}

interface AuthContextType {
  // State
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  authMode: "local" | "oidc";

  // Local auth methods
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;

  // OIDC methods
  loginWithOIDC: () => void;
  handleOAuthCallback: (code: string, state?: string) => Promise<void>;

  // Common methods
  logout: () => Promise<void>;
  refreshTokens: () => Promise<boolean>;

  // Token access (for API calls)
  getAccessToken: () => string | null;
  // IDP token access (for calling IDP-specific APIs like OAuth account linking)
  getIDPToken: () => string | null;
}

// =============================================================================
// CONFIGURATION
// =============================================================================

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:4200";
const AUTH_MODE = (process.env.NEXT_PUBLIC_AUTH_MODE || "local") as
  | "local"
  | "oidc";

// Token refresh buffer - refresh 5 minutes before expiry
const TOKEN_REFRESH_BUFFER_MS = 5 * 60 * 1000;

/**
 * Safely parse error response, handling both JSON and plain text responses.
 * Returns the error message from the response body.
 */
async function parseErrorResponse(
  response: Response,
  fallbackMessage: string,
): Promise<string> {
  const contentType = response.headers.get("content-type");
  const text = await response.text();

  // Try to parse as JSON if content-type suggests it or if it looks like JSON
  if (contentType?.includes("application/json") || text.startsWith("{")) {
    try {
      const json = JSON.parse(text);
      return json.error || json.message || json.detail || fallbackMessage;
    } catch {
      // JSON parsing failed, fall through to use text
    }
  }

  // Return plain text error if not empty, otherwise fallback
  return text.trim() || fallbackMessage;
}

// =============================================================================
// CONTEXT
// =============================================================================

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// =============================================================================
// STORAGE HELPERS
// =============================================================================

const storage = {
  getTokens(): TokenInfo | null {
    if (typeof window === "undefined") return null;
    const stored = localStorage.getItem("enclii_tokens");
    if (!stored) return null;
    try {
      return JSON.parse(stored);
    } catch {
      return null;
    }
  },

  setTokens(tokens: TokenInfo): void {
    if (typeof window === "undefined") return;
    localStorage.setItem("enclii_tokens", JSON.stringify(tokens));
  },

  clearTokens(): void {
    if (typeof window === "undefined") return;
    localStorage.removeItem("enclii_tokens");
  },

  getUser(): User | null {
    if (typeof window === "undefined") return null;
    const stored = localStorage.getItem("enclii_user");
    if (!stored) return null;
    try {
      return JSON.parse(stored);
    } catch {
      return null;
    }
  },

  setUser(user: User): void {
    if (typeof window === "undefined") return;
    localStorage.setItem("enclii_user", JSON.stringify(user));
  },

  clearUser(): void {
    if (typeof window === "undefined") return;
    localStorage.removeItem("enclii_user");
  },

  clear(): void {
    this.clearTokens();
    this.clearUser();
  },
};

// =============================================================================
// JWT HELPERS
// =============================================================================

function parseJwt(token: string): Record<string, unknown> | null {
  try {
    const base64Url = token.split(".")[1];
    const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split("")
        .map((c) => "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2))
        .join(""),
    );
    return JSON.parse(jsonPayload);
  } catch {
    return null;
  }
}

function isTokenExpired(expiresAt: number): boolean {
  return Date.now() >= expiresAt - TOKEN_REFRESH_BUFFER_MS;
}

// =============================================================================
// PROVIDER
// =============================================================================

interface AuthProviderProps {
  children: ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [user, setUser] = useState<User | null>(null);
  const [tokens, setTokens] = useState<TokenInfo | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [refreshTimer, setRefreshTimer] = useState<NodeJS.Timeout | null>(null);

  // ==========================================================================
  // INITIALIZATION
  // ==========================================================================

  useEffect(() => {
    // Load stored auth state on mount
    const storedTokens = storage.getTokens();
    const storedUser = storage.getUser();

    if (storedTokens && storedUser) {
      if (!isTokenExpired(storedTokens.expiresAt)) {
        setTokens(storedTokens);
        setUser(storedUser);
        scheduleTokenRefresh(storedTokens.expiresAt);
      } else if (storedTokens.refreshToken) {
        // Token expired but we have refresh token - try to refresh
        refreshTokens().catch(() => {
          storage.clear();
        });
      } else {
        storage.clear();
      }
    }

    setIsLoading(false);
  }, []);

  // ==========================================================================
  // TOKEN REFRESH SCHEDULING
  // ==========================================================================

  const scheduleTokenRefresh = useCallback(
    (expiresAt: number) => {
      // Clear existing timer
      if (refreshTimer) {
        clearTimeout(refreshTimer);
      }

      // Calculate when to refresh (5 minutes before expiry)
      const refreshIn = expiresAt - Date.now() - TOKEN_REFRESH_BUFFER_MS;

      if (refreshIn > 0) {
        const timer = setTimeout(() => {
          refreshTokens();
        }, refreshIn);
        setRefreshTimer(timer);
      }
    },
    [refreshTimer],
  );

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (refreshTimer) {
        clearTimeout(refreshTimer);
      }
    };
  }, [refreshTimer]);

  // ==========================================================================
  // API HELPERS
  // ==========================================================================

  const apiRequest = async (
    endpoint: string,
    options: RequestInit = {},
  ): Promise<Response> => {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    };

    if (tokens?.accessToken) {
      headers["Authorization"] = `Bearer ${tokens.accessToken}`;
    }

    return fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers,
    });
  };

  // ==========================================================================
  // LOCAL AUTHENTICATION
  // ==========================================================================

  const login = async (email: string, password: string): Promise<void> => {
    setIsLoading(true);

    try {
      const response = await apiRequest("/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const errorMessage = await parseErrorResponse(response, "Login failed");
        throw new Error(errorMessage);
      }

      const data = await response.json();

      const tokenInfo: TokenInfo = {
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
        expiresAt: new Date(data.expires_at).getTime(),
        tokenType: data.token_type || "Bearer",
      };

      const userData: User = {
        id: data.user?.id || "",
        email: data.user?.email || email,
        name: data.user?.name,
        roles: data.user?.roles || [],
      };

      setTokens(tokenInfo);
      setUser(userData);
      storage.setTokens(tokenInfo);
      storage.setUser(userData);
      scheduleTokenRefresh(tokenInfo.expiresAt);
    } finally {
      setIsLoading(false);
    }
  };

  const register = async (
    email: string,
    password: string,
    name: string,
  ): Promise<void> => {
    setIsLoading(true);

    try {
      const response = await apiRequest("/v1/auth/register", {
        method: "POST",
        body: JSON.stringify({ email, password, name }),
      });

      if (!response.ok) {
        const errorMessage = await parseErrorResponse(
          response,
          "Registration failed",
        );
        throw new Error(errorMessage);
      }

      const data = await response.json();

      const tokenInfo: TokenInfo = {
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
        expiresAt: new Date(data.expires_at).getTime(),
        tokenType: data.token_type || "Bearer",
      };

      const userData: User = {
        id: data.user?.id || "",
        email: data.user?.email || email,
        name: data.user?.name || name,
        roles: data.user?.roles || [],
      };

      setTokens(tokenInfo);
      setUser(userData);
      storage.setTokens(tokenInfo);
      storage.setUser(userData);
      scheduleTokenRefresh(tokenInfo.expiresAt);
    } finally {
      setIsLoading(false);
    }
  };

  // ==========================================================================
  // OIDC AUTHENTICATION
  // ==========================================================================

  const loginWithOIDC = (): void => {
    // Store current URL for redirect after login
    if (typeof window !== "undefined") {
      localStorage.setItem("auth_return_url", window.location.pathname);
    }

    // Redirect to backend OIDC login endpoint
    // The backend will redirect to the OIDC provider (Janua)
    // Note: In OIDC mode, the API registers GET /v1/auth/login for OIDC redirect
    window.location.href = `${API_BASE_URL}/v1/auth/login`;
  };

  const handleOAuthCallback = async (
    code: string,
    state?: string,
  ): Promise<void> => {
    setIsLoading(true);

    try {
      // The backend handles the code exchange, we just need to hit the callback
      // endpoint with the code and state
      const params = new URLSearchParams({ code });
      if (state) {
        params.append("state", state);
      }

      const response = await fetch(
        `${API_BASE_URL}/v1/auth/callback?${params.toString()}`,
        {
          method: "GET",
          credentials: "include", // Include cookies for state verification
        },
      );

      if (!response.ok) {
        const errorMessage = await parseErrorResponse(
          response,
          "OAuth callback failed",
        );
        throw new Error(errorMessage);
      }

      const data = await response.json();

      const tokenInfo: TokenInfo = {
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
        expiresAt: new Date(data.expires_at).getTime(),
        tokenType: data.token_type || "Bearer",
        // Store IDP token for calling IDP-specific APIs (e.g., Janua OAuth linking)
        idpToken: data.idp_token,
        idpTokenExpiresAt: data.idp_token_expires_at
          ? new Date(data.idp_token_expires_at).getTime()
          : undefined,
      };

      // Extract user info from token
      const claims = parseJwt(data.access_token);
      const userData: User = {
        id: (claims?.sub as string) || (claims?.user_id as string) || "",
        email: (claims?.email as string) || "",
        name: claims?.name as string,
        roles: (claims?.roles as string[]) || [],
      };

      setTokens(tokenInfo);
      setUser(userData);
      storage.setTokens(tokenInfo);
      storage.setUser(userData);
      scheduleTokenRefresh(tokenInfo.expiresAt);
    } finally {
      setIsLoading(false);
    }
  };

  // ==========================================================================
  // COMMON METHODS
  // ==========================================================================

  const logout = async (): Promise<void> => {
    try {
      // Call backend logout endpoint to revoke session
      if (tokens?.accessToken) {
        await apiRequest("/v1/auth/logout", {
          method: "POST",
        }).catch(() => {
          // Ignore errors - we're logging out anyway
        });
      }
    } finally {
      // Clear local state regardless of API call result
      setTokens(null);
      setUser(null);
      storage.clear();

      if (refreshTimer) {
        clearTimeout(refreshTimer);
        setRefreshTimer(null);
      }
    }
  };

  const refreshTokens = async (): Promise<boolean> => {
    const currentTokens = tokens || storage.getTokens();

    if (!currentTokens?.refreshToken) {
      return false;
    }

    try {
      const response = await fetch(`${API_BASE_URL}/v1/auth/refresh`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          refresh_token: currentTokens.refreshToken,
        }),
      });

      if (!response.ok) {
        throw new Error("Token refresh failed");
      }

      const data = await response.json();

      const newTokenInfo: TokenInfo = {
        accessToken: data.access_token,
        refreshToken: currentTokens.refreshToken, // Keep existing refresh token
        expiresAt: new Date(data.expires_at).getTime(),
        tokenType: data.token_type || "Bearer",
      };

      setTokens(newTokenInfo);
      storage.setTokens(newTokenInfo);
      scheduleTokenRefresh(newTokenInfo.expiresAt);

      return true;
    } catch (error) {
      console.error("Token refresh failed:", error);
      // Clear auth state on refresh failure
      await logout();
      return false;
    }
  };

  const getAccessToken = (): string | null => {
    const currentTokens = tokens || storage.getTokens();

    if (!currentTokens) {
      return null;
    }

    // Check if token is expired
    if (isTokenExpired(currentTokens.expiresAt)) {
      // Trigger refresh but return current token
      // (the API call might still work if server has grace period)
      refreshTokens();
    }

    return currentTokens.accessToken;
  };

  const getIDPToken = (): string | null => {
    const currentTokens = tokens || storage.getTokens();

    if (!currentTokens?.idpToken) {
      return null;
    }

    // Check if IDP token is expired
    if (
      currentTokens.idpTokenExpiresAt &&
      Date.now() >= currentTokens.idpTokenExpiresAt
    ) {
      // IDP token expired - user needs to re-authenticate
      return null;
    }

    return currentTokens.idpToken;
  };

  // ==========================================================================
  // CONTEXT VALUE
  // ==========================================================================

  const value: AuthContextType = {
    user,
    isAuthenticated: !!user && !!tokens,
    isLoading,
    authMode: AUTH_MODE,
    login,
    register,
    loginWithOIDC,
    handleOAuthCallback,
    logout,
    refreshTokens,
    getAccessToken,
    getIDPToken,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// =============================================================================
// HOOKS
// =============================================================================

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

/**
 * Hook for protecting routes that require authentication.
 * Returns redirect info for unauthenticated users.
 */
export function useRequireAuth(): {
  isAuthenticated: boolean;
  isLoading: boolean;
  shouldRedirect: boolean;
} {
  const { isAuthenticated, isLoading } = useAuth();

  return {
    isAuthenticated,
    isLoading,
    shouldRedirect: !isLoading && !isAuthenticated,
  };
}

/**
 * Hook for getting the access token for API requests.
 * Automatically handles token refresh.
 */
export function useAccessToken(): string | null {
  const { getAccessToken, isAuthenticated } = useAuth();

  if (!isAuthenticated) {
    return null;
  }

  return getAccessToken();
}
