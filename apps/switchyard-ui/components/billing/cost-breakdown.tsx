"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  Legend,
  Tooltip
} from "recharts";
import { cn } from "@/lib/utils";

interface CostCategory {
  name: string;
  value: number;
  color: string;
}

interface CostBreakdownProps {
  projectId: string;
  periodStart?: Date;
  periodEnd?: Date;
  className?: string;
}

const defaultCosts: CostCategory[] = [
  { name: "Compute", value: 12.45, color: "#3b82f6" },
  { name: "Build", value: 4.20, color: "#22c55e" },
  { name: "Storage", value: 2.50, color: "#f59e0b" },
  { name: "Bandwidth", value: 1.85, color: "#8b5cf6" },
];

export function CostBreakdown({
  projectId,
  periodStart,
  periodEnd,
  className
}: CostBreakdownProps) {
  const costs = defaultCosts; // Would fetch from API
  const totalCost = costs.reduce((sum, c) => sum + c.value, 0);
  const planBase = 20.00; // Pro plan
  const grandTotal = planBase + totalCost;

  return (
    <Card className={cn("", className)}>
      <CardHeader>
        <CardTitle className="text-sm font-medium">
          Cost Breakdown
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          Current billing period
        </p>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between mb-6">
          <div>
            <p className="text-3xl font-bold">${grandTotal.toFixed(2)}</p>
            <p className="text-sm text-muted-foreground">
              ${planBase.toFixed(2)} base + ${totalCost.toFixed(2)} usage
            </p>
          </div>

          <div className="h-32 w-32">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={costs}
                  cx="50%"
                  cy="50%"
                  innerRadius={35}
                  outerRadius={50}
                  paddingAngle={2}
                  dataKey="value"
                >
                  {costs.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip
                  formatter={(value: number) => [`$${value.toFixed(2)}`, ""]}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="space-y-3">
          <div className="flex items-center justify-between py-2 border-b">
            <span className="text-sm font-medium">Pro Plan (base)</span>
            <span className="font-mono">${planBase.toFixed(2)}</span>
          </div>

          {costs.map((cost) => (
            <div
              key={cost.name}
              className="flex items-center justify-between py-1"
            >
              <div className="flex items-center gap-2">
                <div
                  className="w-3 h-3 rounded-full"
                  style={{ backgroundColor: cost.color }}
                />
                <span className="text-sm">{cost.name}</span>
              </div>
              <span className="font-mono text-sm">
                ${cost.value.toFixed(2)}
              </span>
            </div>
          ))}

          <div className="flex items-center justify-between pt-3 border-t">
            <span className="font-medium">Estimated Total</span>
            <span className="font-mono font-bold">
              ${grandTotal.toFixed(2)}
            </span>
          </div>
        </div>

        <p className="text-xs text-muted-foreground mt-4">
          Final invoice generated on the 1st of each month
        </p>
      </CardContent>
    </Card>
  );
}
