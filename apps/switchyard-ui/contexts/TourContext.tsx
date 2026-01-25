'use client';

import React, { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react';
import { useTier } from '@/hooks/use-tier';

// =============================================================================
// TYPES
// =============================================================================

interface TourContextType {
  /** Whether the tour has been completed */
  tourCompleted: boolean;
  /** Whether the tour should be shown (paid user + not completed) */
  shouldShowTour: boolean;
  /** Mark the tour as completed */
  completeTour: () => void;
  /** Reset the tour (for testing/debugging) */
  resetTour: () => void;
  /** Start the tour manually */
  startTour: () => void;
  /** Whether the tour is currently active */
  isTourActive: boolean;
  /** Set the tour active state */
  setTourActive: (active: boolean) => void;
}

// =============================================================================
// CONSTANTS
// =============================================================================

const TOUR_STORAGE_KEY = 'enclii_tour_completed';

// =============================================================================
// CONTEXT
// =============================================================================

const TourContext = createContext<TourContextType | undefined>(undefined);

// =============================================================================
// PROVIDER
// =============================================================================

interface TourProviderProps {
  children: ReactNode;
}

export function TourProvider({ children }: TourProviderProps) {
  const { isPaid } = useTier();
  const [tourCompleted, setTourCompleted] = useState(true); // Default to true to prevent flash
  const [isTourActive, setTourActive] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);

  // Initialize from localStorage on mount
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem(TOUR_STORAGE_KEY);
      setTourCompleted(stored === 'true');
      setIsInitialized(true);
    }
  }, []);

  // Determine if tour should be shown
  const shouldShowTour = isInitialized && isPaid && !tourCompleted;

  // Mark tour as completed
  const completeTour = useCallback(() => {
    setTourCompleted(true);
    setTourActive(false);
    if (typeof window !== 'undefined') {
      localStorage.setItem(TOUR_STORAGE_KEY, 'true');
    }
  }, []);

  // Reset tour (for testing)
  const resetTour = useCallback(() => {
    setTourCompleted(false);
    if (typeof window !== 'undefined') {
      localStorage.removeItem(TOUR_STORAGE_KEY);
    }
  }, []);

  // Start tour manually
  const startTour = useCallback(() => {
    setTourActive(true);
  }, []);

  const value: TourContextType = {
    tourCompleted,
    shouldShowTour,
    completeTour,
    resetTour,
    startTour,
    isTourActive,
    setTourActive,
  };

  return <TourContext.Provider value={value}>{children}</TourContext.Provider>;
}

// =============================================================================
// HOOK
// =============================================================================

export function useTour(): TourContextType {
  const context = useContext(TourContext);
  if (context === undefined) {
    throw new Error('useTour must be used within a TourProvider');
  }
  return context;
}
