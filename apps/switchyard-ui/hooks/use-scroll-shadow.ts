'use client';

import { useState, useEffect, useCallback } from 'react';

interface UseScrollShadowOptions {
  /** Scroll threshold to trigger shadow (in pixels) */
  threshold?: number;
  /** Target element selector (defaults to window) */
  targetSelector?: string;
}

interface UseScrollShadowReturn {
  /** Whether the shadow should be visible */
  isScrolled: boolean;
  /** Current scroll position */
  scrollY: number;
  /** CSS class for shadow effect */
  shadowClass: string;
}

/**
 * Hook to detect scroll position and provide shadow effect for sticky headers
 * Follows Vercel's pattern of subtle shadow on scroll
 */
export function useScrollShadow({
  threshold = 10,
  targetSelector,
}: UseScrollShadowOptions = {}): UseScrollShadowReturn {
  const [isScrolled, setIsScrolled] = useState(false);
  const [scrollY, setScrollY] = useState(0);

  const handleScroll = useCallback(() => {
    let currentScrollY: number;

    if (targetSelector) {
      const element = document.querySelector(targetSelector);
      currentScrollY = element?.scrollTop || 0;
    } else {
      currentScrollY = window.scrollY;
    }

    setScrollY(currentScrollY);
    setIsScrolled(currentScrollY > threshold);
  }, [threshold, targetSelector]);

  useEffect(() => {
    // Check initial scroll position on mount
    const checkInitialScroll = () => {
      let currentScrollY: number;
      if (targetSelector) {
        const element = document.querySelector(targetSelector);
        currentScrollY = element?.scrollTop || 0;
      } else {
        currentScrollY = window.scrollY;
      }
      setScrollY(currentScrollY);
      setIsScrolled(currentScrollY > threshold);
    };

    checkInitialScroll();

    // Add scroll listener
    if (targetSelector) {
      const element = document.querySelector(targetSelector);
      if (element) {
        element.addEventListener('scroll', handleScroll, { passive: true });
        return () => element.removeEventListener('scroll', handleScroll);
      }
    } else {
      window.addEventListener('scroll', handleScroll, { passive: true });
      return () => window.removeEventListener('scroll', handleScroll);
    }
  }, [handleScroll, targetSelector]);

  // Shadow class that matches Vercel's subtle shadow aesthetic
  const shadowClass = isScrolled
    ? 'shadow-[0_1px_3px_0_rgba(0,0,0,0.1),0_1px_2px_-1px_rgba(0,0,0,0.1)] dark:shadow-[0_1px_3px_0_rgba(0,0,0,0.3),0_1px_2px_-1px_rgba(0,0,0,0.2)]'
    : '';

  return {
    isScrolled,
    scrollY,
    shadowClass,
  };
}

/**
 * Simpler hook that just returns whether page is scrolled
 */
export function useIsScrolled(threshold = 10): boolean {
  const [isScrolled, setIsScrolled] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > threshold);
    };

    handleScroll(); // Check initial position
    window.addEventListener('scroll', handleScroll, { passive: true });
    return () => window.removeEventListener('scroll', handleScroll);
  }, [threshold]);

  return isScrolled;
}
