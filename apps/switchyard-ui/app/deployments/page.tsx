'use client';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function DeploymentsPage() {
  return (
    <div className="container mx-auto py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold">Deployments</h1>
        <p className="text-muted-foreground mt-2">
          Track and manage your deployment history
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Deployment History</CardTitle>
          <CardDescription>
            View all deployments across your services
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-center py-12 text-muted-foreground">
            <p className="text-lg">No deployments yet</p>
            <p className="text-sm mt-2">
              Deploy a service to see your deployment history here
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
