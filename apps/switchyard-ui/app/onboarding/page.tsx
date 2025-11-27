import { Metadata } from "next";
import { OnboardingWizard } from "@/components/onboarding/onboarding-wizard";

export const metadata: Metadata = {
  title: "Get Started | Enclii",
  description: "Set up your Enclii account",
};

export default function OnboardingPage() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-background to-muted/20">
      <div className="container max-w-4xl py-12">
        {/* Header */}
        <div className="text-center mb-12">
          <h1 className="text-3xl font-bold tracking-tight mb-2">
            Welcome to Enclii
          </h1>
          <p className="text-muted-foreground">
            Let's get you set up in just a few steps
          </p>
        </div>

        {/* Wizard */}
        <OnboardingWizard />
      </div>
    </div>
  );
}
