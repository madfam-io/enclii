'use client';

import { AuthenticatedLayout } from '@/components/AuthenticatedLayout';

export default function ProtectedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AuthenticatedLayout>{children}</AuthenticatedLayout>;
}
