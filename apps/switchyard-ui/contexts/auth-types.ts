/**
 * Auth Types
 * Type definitions for the authentication context
 */

// =============================================================================
// USER TYPES
// =============================================================================

export interface User {
  id: string;
  email: string;
  name?: string;
  roles?: string[];
  avatarUrl?: string;
  /** Foundry tier from Janua JWT claims (after Dhanam purchase) */
  foundry_tier?: 'community' | 'sovereign' | 'ecosystem' | null;
}

// =============================================================================
// TOKEN TYPES
// =============================================================================

export interface TokenInfo {
  accessToken: string;
  refreshToken?: string;
  expiresAt: number; // Unix timestamp
  tokenType: string;
  // IDP token from identity provider (e.g., Janua) for calling IDP-specific APIs
  idpToken?: string;
  idpTokenExpiresAt?: number; // Unix timestamp
}

export interface RedirectTokens {
  accessToken: string;
  refreshToken: string;
  expiresAt: Date;
  tokenType: string;
  idpToken?: string;
  idpTokenExpiresAt?: Date;
}

// =============================================================================
// CONTEXT TYPES
// =============================================================================

export type AuthMode = "local" | "oidc";

export interface AuthContextType {
  // State
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  authMode: AuthMode;
  /** Error message from auth operations (token refresh, logout, etc.) */
  authError: string | null;
  /** Clear the current auth error */
  clearAuthError: () => void;

  // Local auth methods
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;

  // OIDC methods
  loginWithOIDC: () => void;
  handleOAuthCallback: (code: string, state?: string) => Promise<void>;
  storeTokensFromRedirect: (tokens: RedirectTokens) => Promise<void>;

  // Common methods
  logout: () => Promise<void>;
  refreshTokens: () => Promise<boolean>;

  // Token access (for API calls)
  getAccessToken: () => string | null;
  // IDP token access (for calling IDP-specific APIs like OAuth account linking)
  getIDPToken: () => string | null;
}
