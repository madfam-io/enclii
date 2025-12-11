"use client";

import { useEffect, useState, Suspense, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

/**
 * OAuth Callback Content Component
 * Separated to allow Suspense boundary for useSearchParams
 */
function AuthCallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { handleOAuthCallback } = useAuth();

  const [status, setStatus] = useState<"processing" | "success" | "error">(
    "processing",
  );
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Guard against duplicate callback processing (React 18 Strict Mode, etc.)
  const hasProcessedRef = useRef(false);

  useEffect(() => {
    async function processCallback() {
      // Prevent duplicate processing of OAuth callback
      // OAuth codes are single-use; processing twice causes errors
      if (hasProcessedRef.current) {
        return;
      }
      hasProcessedRef.current = true;

      // Check for error from OIDC provider
      const error = searchParams.get("error");
      const errorDescription = searchParams.get("error_description");

      if (error) {
        console.error("OAuth error:", error, errorDescription);
        setStatus("error");
        setErrorMessage(errorDescription || `Authentication failed: ${error}`);
        return;
      }

      // Get authorization code
      const code = searchParams.get("code");
      const state = searchParams.get("state");

      if (!code) {
        setStatus("error");
        setErrorMessage("No authorization code received from provider");
        return;
      }

      try {
        // Exchange code for tokens via backend
        await handleOAuthCallback(code, state || undefined);

        setStatus("success");

        // Redirect to dashboard after short delay
        setTimeout(() => {
          const returnUrl = localStorage.getItem("auth_return_url") || "/";
          localStorage.removeItem("auth_return_url");
          router.push(returnUrl);
        }, 1500);
      } catch (err) {
        console.error("OAuth callback error:", err);
        setStatus("error");
        setErrorMessage(
          err instanceof Error
            ? err.message
            : "Failed to complete authentication",
        );
      }
    }

    processCallback();
  }, [searchParams, handleOAuthCallback, router]);

  return (
    <div className="text-center">
      {/* Enclii Logo */}
      <h1 className="text-3xl font-bold text-enclii-blue mb-2">🚂 Enclii</h1>
      <p className="text-gray-500 text-sm mb-8">Switchyard Platform</p>

      {status === "processing" && (
        <div className="space-y-4">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-enclii-blue mx-auto"></div>
          <p className="text-gray-600">Completing authentication...</p>
          <p className="text-gray-400 text-sm">
            Please wait while we verify your credentials
          </p>
        </div>
      )}

      {status === "success" && (
        <div className="space-y-4">
          <div className="rounded-full h-12 w-12 bg-green-100 mx-auto flex items-center justify-center">
            <svg
              className="h-6 w-6 text-green-600"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M5 13l4 4L19 7"
              />
            </svg>
          </div>
          <p className="text-green-600 font-medium">
            Authentication successful!
          </p>
          <p className="text-gray-400 text-sm">Redirecting to dashboard...</p>
        </div>
      )}

      {status === "error" && (
        <div className="space-y-4">
          <div className="rounded-full h-12 w-12 bg-red-100 mx-auto flex items-center justify-center">
            <svg
              className="h-6 w-6 text-red-600"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </div>
          <p className="text-red-600 font-medium">Authentication failed</p>
          <p className="text-gray-500 text-sm">{errorMessage}</p>

          <div className="pt-4 space-y-2">
            <button
              onClick={() => router.push("/auth/login")}
              className="w-full bg-enclii-blue text-white py-2 px-4 rounded-md hover:bg-blue-700 transition-colors"
            >
              Try again
            </button>
            <button
              onClick={() => router.push("/")}
              className="w-full bg-gray-100 text-gray-700 py-2 px-4 rounded-md hover:bg-gray-200 transition-colors"
            >
              Return to home
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * Loading fallback for Suspense
 */
function AuthCallbackLoading() {
  return (
    <div className="text-center">
      <h1 className="text-3xl font-bold text-enclii-blue mb-2">🚂 Enclii</h1>
      <p className="text-gray-500 text-sm mb-8">Switchyard Platform</p>
      <div className="space-y-4">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-enclii-blue mx-auto"></div>
        <p className="text-gray-600">Loading...</p>
      </div>
    </div>
  );
}

/**
 * OAuth Callback Page
 *
 * Handles the redirect from the OIDC provider (Janua/Janua) after authentication.
 * Exchanges the authorization code for tokens and establishes the session.
 */
export default function AuthCallbackPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full space-y-8 p-8">
        <Suspense fallback={<AuthCallbackLoading />}>
          <AuthCallbackContent />
        </Suspense>
      </div>
    </div>
  );
}
