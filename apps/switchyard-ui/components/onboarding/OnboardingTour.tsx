'use client';

import { useEffect, useRef } from 'react';
import { driver, type DriveStep, type Driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { useTour } from '@/contexts/TourContext';

// =============================================================================
// TOUR STEPS
// =============================================================================

const TOUR_STEPS: DriveStep[] = [
  {
    popover: {
      title: 'Welcome to Enclii!',
      description:
        "You're now on the Sovereign tier. Let's take a quick tour to help you get started with deploying your first project.",
      side: 'over',
      align: 'center',
    },
  },
  {
    element: '[data-tour="projects"]',
    popover: {
      title: 'Your Projects',
      description:
        'Projects are containers for your services. Think of them as workspaces for your applications.',
      side: 'bottom',
      align: 'start',
    },
  },
  {
    element: '[data-tour="create-project"]',
    popover: {
      title: 'Create Your First Project',
      description:
        'Click here to create a new project. With Sovereign tier, you can create up to 10 projects.',
      side: 'bottom',
      align: 'start',
    },
  },
  {
    element: '[data-tour="services"]',
    popover: {
      title: 'Your Services',
      description:
        'Services are the individual applications you deploy. Each service can be a web app, API, or worker.',
      side: 'bottom',
      align: 'start',
    },
  },
  {
    element: '[data-tour="domains"]',
    popover: {
      title: 'Custom Domains',
      description:
        'Connect your own domains to your services. Auto SSL certificates are included with your Sovereign tier.',
      side: 'bottom',
      align: 'start',
    },
  },
  {
    element: '[data-tour="observability"]',
    popover: {
      title: 'Observability',
      description:
        'Monitor your services with built-in logs, metrics, and traces. Know exactly what your apps are doing.',
      side: 'bottom',
      align: 'start',
    },
  },
  {
    popover: {
      title: "You're All Set!",
      description:
        'Create your first project to start deploying. Need help? Check out our docs at docs.enclii.dev',
      side: 'over',
      align: 'center',
    },
  },
];

// =============================================================================
// COMPONENT
// =============================================================================

export function OnboardingTour() {
  const { shouldShowTour, isTourActive, completeTour, setTourActive } = useTour();
  const driverRef = useRef<Driver | null>(null);

  useEffect(() => {
    // Auto-start tour for new paid users
    if (shouldShowTour && !isTourActive) {
      // Small delay to ensure DOM is ready
      const timer = setTimeout(() => {
        setTourActive(true);
      }, 500);
      return () => clearTimeout(timer);
    }
  }, [shouldShowTour, isTourActive, setTourActive]);

  useEffect(() => {
    if (!isTourActive) {
      // Cleanup if tour is deactivated
      if (driverRef.current) {
        driverRef.current.destroy();
        driverRef.current = null;
      }
      return;
    }

    // Initialize driver.js
    const driverObj = driver({
      showProgress: true,
      showButtons: ['next', 'previous', 'close'],
      steps: TOUR_STEPS,
      nextBtnText: 'Next',
      prevBtnText: 'Previous',
      doneBtnText: 'Get Started',
      progressText: '{{current}} of {{total}}',
      popoverClass: 'enclii-tour-popover',
      onDestroyStarted: () => {
        completeTour();
      },
      onCloseClick: () => {
        completeTour();
        driverObj.destroy();
      },
    });

    driverRef.current = driverObj;
    driverObj.drive();

    return () => {
      if (driverRef.current) {
        driverRef.current.destroy();
        driverRef.current = null;
      }
    };
  }, [isTourActive, completeTour]);

  // This component doesn't render anything visible
  // It just manages the driver.js instance
  return null;
}

export default OnboardingTour;
