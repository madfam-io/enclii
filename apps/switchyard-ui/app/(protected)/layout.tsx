'use client';

import { AuthenticatedLayout } from '@/components/AuthenticatedLayout';
import { TourProvider } from '@/contexts/TourContext';
import { OnboardingTour } from '@/components/onboarding/OnboardingTour';

export default function ProtectedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <TourProvider>
      <AuthenticatedLayout>{children}</AuthenticatedLayout>
      <OnboardingTour />
    </TourProvider>
  );
}
