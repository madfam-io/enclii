'use client';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function ServicesPage() {
  return (
    <div className="container mx-auto py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold">Services</h1>
        <p className="text-muted-foreground mt-2">
          Manage and monitor your deployed services
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Services Overview</CardTitle>
          <CardDescription>
            View all services across your projects
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-center py-12 text-muted-foreground">
            <p className="text-lg">No services deployed yet</p>
            <p className="text-sm mt-2">
              Create a project and deploy services to see them here
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
