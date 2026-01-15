'use client';

import { useState, useEffect, useCallback, useRef } from 'react';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:4200';
// Skip silent auth in E2E tests or when explicitly disabled
const SKIP_SILENT_AUTH = process.env.NEXT_PUBLIC_SKIP_SILENT_AUTH === 'true';

interface SilentAuthResult {
  type: 'silent-auth-result';
  success: boolean;
  access_token?: string;
  refresh_token?: string;
  expires_at?: number;
  token_type?: string;
  idp_token?: string;
  idp_token_expires_at?: number;
  error?: string;
  error_description?: string;
}

interface UseSilentAuthReturn {
  isChecking: boolean;
  hasValidSession: boolean;
  tokens: SilentAuthResult | null;
  error: string | null;
  checkSilentAuth: () => Promise<void>;
}

/**
 * Hook for checking if the user has a valid SSO session via silent authentication.
 * Uses an iframe with prompt=none to check if the OIDC provider has an active session.
 *
 * Flow:
 * 1. Call /v1/auth/silent-check to get the silent auth URL
 * 2. Open the URL in a hidden iframe
 * 3. Listen for postMessage from the iframe with the result
 * 4. If successful, receive tokens; if not, user needs to log in
 */
export function useSilentAuth(): UseSilentAuthReturn {
  const [isChecking, setIsChecking] = useState(true);
  const [hasValidSession, setHasValidSession] = useState(false);
  const [tokens, setTokens] = useState<SilentAuthResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  const cleanup = useCallback(() => {
    if (iframeRef.current) {
      document.body.removeChild(iframeRef.current);
      iframeRef.current = null;
    }
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
  }, []);

  const checkSilentAuth = useCallback(async () => {
    // Skip silent auth in E2E tests
    if (SKIP_SILENT_AUTH) {
      setIsChecking(false);
      setHasValidSession(false);
      return;
    }

    setIsChecking(true);
    setError(null);
    setHasValidSession(false);
    setTokens(null);

    try {
      // Step 1: Get silent auth URL from backend with timeout
      const controller = new AbortController();
      const fetchTimeout = setTimeout(() => controller.abort(), 3000); // 3 second timeout

      let response: Response;
      try {
        response = await fetch(`${API_URL}/v1/auth/silent-check`, {
          method: 'GET',
          credentials: 'include', // Include cookies for state cookie
          signal: controller.signal,
        });
      } catch (fetchError) {
        clearTimeout(fetchTimeout);
        // Network error or timeout - API unavailable, proceed without silent auth
        if (fetchError instanceof Error && fetchError.name === 'AbortError') {
          console.debug('Silent auth check timed out - API may be unavailable');
        } else {
          console.debug('Silent auth check failed - API may be unavailable:', fetchError);
        }
        setHasValidSession(false);
        setIsChecking(false);
        return;
      }
      clearTimeout(fetchTimeout);

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to get silent auth URL');
      }

      const { auth_url } = await response.json();

      // Step 2: Create hidden iframe
      cleanup(); // Clean up any existing iframe

      const iframe = document.createElement('iframe');
      iframe.style.display = 'none';
      iframe.style.width = '0';
      iframe.style.height = '0';
      iframe.style.border = 'none';
      iframe.style.position = 'absolute';
      iframe.style.left = '-9999px';
      document.body.appendChild(iframe);
      iframeRef.current = iframe;

      // Step 3: Set up message listener for postMessage from iframe
      const messageHandler = (event: MessageEvent) => {
        // Only accept messages from our origin
        if (event.origin !== window.location.origin) {
          return;
        }

        const data = event.data as SilentAuthResult;
        if (data?.type !== 'silent-auth-result') {
          return;
        }

        // Clean up
        window.removeEventListener('message', messageHandler);
        cleanup();

        if (data.success) {
          setHasValidSession(true);
          setTokens(data);
        } else {
          // login_required, interaction_required are expected for unauthenticated users
          const isExpectedError = ['login_required', 'interaction_required', 'consent_required'].includes(data.error || '');
          if (!isExpectedError) {
            setError(data.error_description || data.error || 'Silent auth failed');
          }
          setHasValidSession(false);
        }
        setIsChecking(false);
      };

      window.addEventListener('message', messageHandler);

      // Step 4: Set timeout for iframe load (5 seconds)
      timeoutRef.current = setTimeout(() => {
        window.removeEventListener('message', messageHandler);
        cleanup();
        setError('Silent auth timeout');
        setHasValidSession(false);
        setIsChecking(false);
      }, 5000);

      // Step 5: Navigate iframe to auth URL
      iframe.src = auth_url;

    } catch (err) {
      cleanup();
      const message = err instanceof Error ? err.message : 'Silent auth failed';
      // Don't set error for OIDC not enabled - just means we're in local mode
      if (!message.includes('not enabled')) {
        setError(message);
      }
      setHasValidSession(false);
      setIsChecking(false);
    }
  }, [cleanup]);

  // Run silent auth check on mount
  useEffect(() => {
    checkSilentAuth();

    return () => {
      cleanup();
    };
  }, [checkSilentAuth, cleanup]);

  return {
    isChecking,
    hasValidSession,
    tokens,
    error,
    checkSilentAuth,
  };
}
