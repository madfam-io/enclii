'use client';

import { useAuth } from '@/contexts/AuthContext';
import { AlertCircle, X } from 'lucide-react';

/**
 * AuthErrorBanner displays auth-related errors (session expiry, token refresh failures, etc.)
 * Should be placed near the top of the authenticated layout.
 */
export function AuthErrorBanner() {
  const { authError, clearAuthError } = useAuth();

  if (!authError) {
    return null;
  }

  return (
    <div className="bg-red-50 dark:bg-red-900/20 border-b border-red-200 dark:border-red-800">
      <div className="max-w-7xl mx-auto px-4 py-3 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="h-5 w-5 text-red-500 dark:text-red-400 flex-shrink-0" />
            <p className="text-sm text-red-700 dark:text-red-300">{authError}</p>
          </div>
          <button
            onClick={clearAuthError}
            className="flex-shrink-0 p-1 rounded hover:bg-red-100 dark:hover:bg-red-800/50 transition-colors"
            aria-label="Dismiss"
          >
            <X className="h-4 w-4 text-red-500 dark:text-red-400" />
          </button>
        </div>
      </div>
    </div>
  );
}
