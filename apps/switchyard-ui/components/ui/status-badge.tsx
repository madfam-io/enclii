import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

/**
 * StatusBadge - Theme-aware status indicator component
 *
 * Uses semantic status colors that automatically adapt to:
 * - Enterprise Light mode
 * - Enterprise Dark mode
 * - Solarpunk theme
 *
 * Status types: success, warning, error, info, neutral
 * Appearance: solid (high emphasis) or muted (soft background)
 */
const statusBadgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      status: {
        success: "",
        warning: "",
        error: "",
        info: "",
        neutral: "",
      },
      appearance: {
        solid: "",
        muted: "",
      },
    },
    compoundVariants: [
      // Success variants
      {
        status: "success",
        appearance: "solid",
        className:
          "border-transparent bg-status-success text-status-success-foreground",
      },
      {
        status: "success",
        appearance: "muted",
        className:
          "border-status-success/20 bg-status-success-muted text-status-success-muted-foreground",
      },
      // Warning variants
      {
        status: "warning",
        appearance: "solid",
        className:
          "border-transparent bg-status-warning text-status-warning-foreground",
      },
      {
        status: "warning",
        appearance: "muted",
        className:
          "border-status-warning/20 bg-status-warning-muted text-status-warning-muted-foreground",
      },
      // Error variants
      {
        status: "error",
        appearance: "solid",
        className:
          "border-transparent bg-status-error text-status-error-foreground",
      },
      {
        status: "error",
        appearance: "muted",
        className:
          "border-status-error/20 bg-status-error-muted text-status-error-muted-foreground",
      },
      // Info variants
      {
        status: "info",
        appearance: "solid",
        className:
          "border-transparent bg-status-info text-status-info-foreground",
      },
      {
        status: "info",
        appearance: "muted",
        className:
          "border-status-info/20 bg-status-info-muted text-status-info-muted-foreground",
      },
      // Neutral variants
      {
        status: "neutral",
        appearance: "solid",
        className:
          "border-transparent bg-status-neutral text-status-neutral-foreground",
      },
      {
        status: "neutral",
        appearance: "muted",
        className:
          "border-status-neutral/20 bg-status-neutral-muted text-status-neutral-muted-foreground",
      },
    ],
    defaultVariants: {
      status: "neutral",
      appearance: "muted",
    },
  }
)

export interface StatusBadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof statusBadgeVariants> {
  /** Optional icon to display before the label */
  icon?: React.ReactNode
}

function StatusBadge({
  className,
  status,
  appearance,
  icon,
  children,
  ...props
}: StatusBadgeProps) {
  return (
    <div
      className={cn(statusBadgeVariants({ status, appearance }), className)}
      {...props}
    >
      {icon && <span className="mr-1 -ml-0.5">{icon}</span>}
      {children}
    </div>
  )
}

export { StatusBadge, statusBadgeVariants }
