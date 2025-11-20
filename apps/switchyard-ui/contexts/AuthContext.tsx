'use client';

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';

/**
 * Authentication Context for managing user authentication state
 *
 * SECURITY WARNING: This is a basic implementation for development.
 * For production, implement OAuth 2.0 / OIDC with providers like:
 * - Auth0 (https://auth0.com/)
 * - Okta (https://www.okta.com/)
 * - Keycloak (https://www.keycloak.org/)
 * - AWS Cognito (https://aws.amazon.com/cognito/)
 *
 * TODO Phase 2:
 * - Integrate OAuth 2.0 provider
 * - Implement token refresh logic
 * - Add session management
 * - Implement secure token storage
 */

interface User {
  id: string;
  email: string;
  name?: string;
  roles?: string[];
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  setToken: (token: string) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

interface AuthProviderProps {
  children: ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setTokenState] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Load token from localStorage on mount
  useEffect(() => {
    const storedToken = localStorage.getItem('auth_token');
    if (storedToken) {
      setTokenState(storedToken);
      // TODO: Validate token and fetch user info
      // For now, decode JWT to get user info (insecure - just for demo)
      try {
        const payload = JSON.parse(atob(storedToken.split('.')[1]));
        setUser({
          id: payload.sub || '',
          email: payload.email || '',
          name: payload.name,
          roles: payload.roles || [],
        });
      } catch (error) {
        console.error('Failed to parse token:', error);
        localStorage.removeItem('auth_token');
      }
    }
    setIsLoading(false);
  }, []);

  const login = async (email: string, password: string) => {
    setIsLoading(true);
    try {
      // TODO: Replace with actual OAuth 2.0 / OIDC login flow
      // This is a placeholder for development only
      const response = await fetch('http://localhost:8080/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        throw new Error('Login failed');
      }

      const data = await response.json();

      if (data.token) {
        setTokenState(data.token);
        localStorage.setItem('auth_token', data.token);

        // Decode token to get user info
        const payload = JSON.parse(atob(data.token.split('.')[1]));
        setUser({
          id: payload.sub || '',
          email: payload.email || email,
          name: payload.name,
          roles: payload.roles || [],
        });
      }
    } catch (error) {
      console.error('Login error:', error);
      throw error;
    } finally {
      setIsLoading(false);
    }
  };

  const logout = () => {
    setUser(null);
    setTokenState(null);
    localStorage.removeItem('auth_token');

    // TODO: Call logout endpoint to invalidate session
    // fetch('http://localhost:8080/api/v1/auth/logout', { method: 'POST' });
  };

  const setToken = (newToken: string) => {
    setTokenState(newToken);
    localStorage.setItem('auth_token', newToken);

    // Decode token to get user info
    try {
      const payload = JSON.parse(atob(newToken.split('.')[1]));
      setUser({
        id: payload.sub || '',
        email: payload.email || '',
        name: payload.name,
        roles: payload.roles || [],
      });
    } catch (error) {
      console.error('Failed to parse token:', error);
    }
  };

  const value: AuthContextType = {
    user,
    token,
    isAuthenticated: !!user && !!token,
    isLoading,
    login,
    logout,
    setToken,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

/**
 * Hook to protect routes that require authentication
 */
export function useRequireAuth() {
  const { isAuthenticated, isLoading } = useAuth();
  const [shouldRedirect, setShouldRedirect] = useState(false);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      setShouldRedirect(true);
    }
  }, [isAuthenticated, isLoading]);

  return { shouldRedirect, isLoading };
}
