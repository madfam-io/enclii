"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { useSilentAuth } from "@/hooks/useSilentAuth";

export default function LoginPage() {
  const router = useRouter();
  const { login, loginWithOIDC, isAuthenticated, isLoading, authMode, storeTokensFromRedirect } = useAuth();
  const { isChecking: isSilentAuthChecking, hasValidSession, tokens: silentAuthTokens } = useSilentAuth();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isProcessingSilentAuth, setIsProcessingSilentAuth] = useState(false);

  // Redirect if already authenticated
  useEffect(() => {
    if (isAuthenticated && !isLoading) {
      router.push("/");
    }
  }, [isAuthenticated, isLoading, router]);

  // Handle successful silent auth - store tokens and redirect
  useEffect(() => {
    if (hasValidSession && silentAuthTokens && !isProcessingSilentAuth) {
      setIsProcessingSilentAuth(true);

      // Convert silent auth tokens to the format expected by storeTokensFromRedirect
      const tokens = {
        accessToken: silentAuthTokens.access_token!,
        refreshToken: silentAuthTokens.refresh_token!,
        expiresAt: new Date(silentAuthTokens.expires_at! * 1000),
        tokenType: silentAuthTokens.token_type || 'Bearer',
        idpToken: silentAuthTokens.idp_token,
        idpTokenExpiresAt: silentAuthTokens.idp_token_expires_at
          ? new Date(silentAuthTokens.idp_token_expires_at * 1000)
          : undefined,
      };

      storeTokensFromRedirect(tokens)
        .then(() => {
          router.push("/");
        })
        .catch((err) => {
          console.error("Failed to store silent auth tokens:", err);
          setIsProcessingSilentAuth(false);
        });
    }
  }, [hasValidSession, silentAuthTokens, storeTokensFromRedirect, router, isProcessingSilentAuth]);

  const handleLocalLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      await login(email, password);
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleOIDCLogin = () => {
    loginWithOIDC();
  };

  // Determine if we're in a loading state
  // NOTE: We no longer block on isSilentAuthChecking - show login form immediately
  // while silent auth runs in background. If successful, useEffect handles redirect.
  const showLoading = isLoading || isProcessingSilentAuth;

  // Show loading only for initial auth check or when processing silent auth redirect
  if (showLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-enclii-blue mx-auto"></div>
          <p className="mt-4 text-gray-600">
            {isProcessingSilentAuth ? "Signing you in..." : "Loading..."}
          </p>
        </div>
      </div>
    );
  }

  // Already authenticated - will redirect
  if (isAuthenticated) {
    return null;
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        {/* Header */}
        <div className="text-center">
          <h1 className="text-4xl font-bold text-enclii-blue mb-2">🚂 Enclii</h1>
          <p className="text-gray-500 text-sm mb-6">Switchyard Platform</p>
          <h2 className="text-2xl font-semibold text-gray-900">
            Sign in to your account
          </h2>
        </div>

        {/* Error message */}
        {error && (
          <div className="bg-status-error-muted border border-status-error/30 rounded-md p-4">
            <div className="flex">
              <div className="flex-shrink-0">
                <svg
                  className="h-5 w-5 text-status-error"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                    clipRule="evenodd"
                  />
                </svg>
              </div>
              <div className="ml-3">
                <p className="text-sm text-status-error-foreground">{error}</p>
              </div>
            </div>
          </div>
        )}

        {/* OIDC Login (Primary for production) */}
        {authMode === "oidc" ? (
          <div className="space-y-6">
            {/* Subtle indicator while silent auth checks for existing session */}
            {isSilentAuthChecking && (
              <div className="text-center text-xs text-gray-400 animate-pulse">
                Checking for existing session...
              </div>
            )}

            <button
              onClick={handleOIDCLogin}
              className="w-full flex justify-center items-center gap-2 py-3 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-enclii-blue hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-enclii-blue transition-colors"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
              Sign in with Janua SSO
            </button>

            <div className="text-center text-sm text-gray-500">
              <p>
                You will be redirected to your organization's identity provider.
              </p>
            </div>
          </div>
        ) : (
          /* Local Login Form (Bootstrap mode) */
          <form className="mt-8 space-y-6" onSubmit={handleLocalLogin}>
            <div className="space-y-4">
              <div>
                <label htmlFor="email" className="block text-sm font-medium text-gray-700">
                  Email address
                </label>
                <input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-enclii-blue focus:border-enclii-blue focus:z-10 sm:text-sm"
                  placeholder="you@example.com"
                />
              </div>

              <div>
                <label htmlFor="password" className="block text-sm font-medium text-gray-700">
                  Password
                </label>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="mt-1 appearance-none relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 rounded-md focus:outline-none focus:ring-enclii-blue focus:border-enclii-blue focus:z-10 sm:text-sm"
                  placeholder="••••••••"
                />
              </div>
            </div>

            <div>
              <button
                type="submit"
                disabled={isSubmitting}
                className="w-full flex justify-center py-3 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-enclii-blue hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-enclii-blue disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {isSubmitting ? (
                  <>
                    <svg
                      className="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      />
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      />
                    </svg>
                    Signing in...
                  </>
                ) : (
                  "Sign in"
                )}
              </button>
            </div>

            <div className="text-center">
              <a
                href="/register"
                className="text-sm text-enclii-blue hover:text-blue-700"
              >
                Don't have an account? Register
              </a>
            </div>
          </form>
        )}

        {/* Footer */}
        <div className="text-center text-xs text-gray-400 mt-8">
          <p>© {new Date().getFullYear()} Enclii Platform. Built with ❤️ for developers.</p>
        </div>
      </div>
    </div>
  );
}
