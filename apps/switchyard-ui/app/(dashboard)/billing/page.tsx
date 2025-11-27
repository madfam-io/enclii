import { Metadata } from "next";
import { UsageMeters } from "@/components/billing/usage-meters";
import { CostBreakdown } from "@/components/billing/cost-breakdown";
import { InvoiceTable } from "@/components/billing/invoice-table";
import { PlanSelector } from "@/components/billing/plan-selector";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CreditCard, Receipt, Settings } from "lucide-react";

export const metadata: Metadata = {
  title: "Billing | Enclii",
  description: "Manage your billing and subscription",
};

export default function BillingPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Billing</h1>
          <p className="text-muted-foreground">
            Manage your subscription and view usage
          </p>
        </div>
        <Button variant="outline">
          <CreditCard className="h-4 w-4 mr-2" />
          Payment Methods
        </Button>
      </div>

      <Tabs defaultValue="overview" className="space-y-6">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="usage">Usage</TabsTrigger>
          <TabsTrigger value="invoices">Invoices</TabsTrigger>
          <TabsTrigger value="plan">Plan</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-6">
          {/* Current Plan Summary */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Pro Plan</CardTitle>
                <p className="text-sm text-muted-foreground">
                  $20/month + usage
                </p>
              </div>
              <Button variant="outline" size="sm">
                <Settings className="h-4 w-4 mr-2" />
                Manage Plan
              </Button>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-1">
                  <p className="text-sm text-muted-foreground">Billing Period</p>
                  <p className="font-medium">Dec 1 - Dec 31, 2024</p>
                </div>
                <div className="space-y-1">
                  <p className="text-sm text-muted-foreground">Next Invoice</p>
                  <p className="font-medium">Jan 1, 2025</p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Usage and Cost Side by Side */}
          <div className="grid gap-6 md:grid-cols-2">
            <UsageMeters projectId="current" />
            <CostBreakdown projectId="current" />
          </div>

          {/* Recent Invoices */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">Recent Invoices</CardTitle>
              <Button variant="ghost" size="sm">
                <Receipt className="h-4 w-4 mr-2" />
                View All
              </Button>
            </CardHeader>
            <CardContent>
              <InvoiceTable invoices={[]} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="usage" className="space-y-6">
          <UsageMeters projectId="current" className="max-w-2xl" />

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Usage History</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-muted-foreground text-sm">
                Detailed usage charts and history coming soon.
              </p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="invoices" className="space-y-6">
          <InvoiceTable invoices={[]} />
        </TabsContent>

        <TabsContent value="plan" className="space-y-6">
          <PlanSelector currentPlanId="pro" />
        </TabsContent>
      </Tabs>
    </div>
  );
}
