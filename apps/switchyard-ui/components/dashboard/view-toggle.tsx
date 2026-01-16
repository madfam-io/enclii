'use client';

import * as React from 'react';
import { Grid3X3, List, LayoutGrid } from 'lucide-react';
import { cn } from '@/lib/utils';

// =============================================================================
// TYPES
// =============================================================================

export type ViewMode = 'grid' | 'list' | 'compact';

interface ViewToggleProps {
  /** Current view mode */
  value: ViewMode;
  /** Callback when view mode changes */
  onChange: (mode: ViewMode) => void;
  /** Available view modes (defaults to grid and list) */
  modes?: ViewMode[];
  /** Size variant */
  size?: 'sm' | 'md';
  /** Additional class names */
  className?: string;
}

// =============================================================================
// VIEW TOGGLE COMPONENT
// =============================================================================

const viewIcons: Record<ViewMode, React.ComponentType<{ className?: string }>> = {
  grid: LayoutGrid,
  list: List,
  compact: Grid3X3,
};

const viewLabels: Record<ViewMode, string> = {
  grid: 'Grid view',
  list: 'List view',
  compact: 'Compact view',
};

export function ViewToggle({
  value,
  onChange,
  modes = ['grid', 'list'],
  size = 'md',
  className,
}: ViewToggleProps) {
  const sizeClasses = {
    sm: {
      container: 'p-0.5',
      button: 'p-1',
      icon: 'h-3.5 w-3.5',
    },
    md: {
      container: 'p-1',
      button: 'p-1.5',
      icon: 'h-4 w-4',
    },
  };

  const sizes = sizeClasses[size];

  return (
    <div
      className={cn(
        'inline-flex items-center rounded-md border border-border bg-muted/50',
        sizes.container,
        className
      )}
      role="radiogroup"
      aria-label="View mode"
    >
      {modes.map((mode) => {
        const Icon = viewIcons[mode];
        const isActive = value === mode;

        return (
          <button
            key={mode}
            type="button"
            role="radio"
            aria-checked={isActive}
            aria-label={viewLabels[mode]}
            onClick={() => onChange(mode)}
            className={cn(
              'rounded transition-colors',
              sizes.button,
              isActive
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
            )}
          >
            <Icon className={sizes.icon} />
          </button>
        );
      })}
    </div>
  );
}

// =============================================================================
// HOOK FOR VIEW MODE STATE
// =============================================================================

interface UseViewModeOptions {
  /** Key for localStorage persistence */
  storageKey?: string;
  /** Default view mode */
  defaultMode?: ViewMode;
}

export function useViewMode({
  storageKey,
  defaultMode = 'grid',
}: UseViewModeOptions = {}): [ViewMode, (mode: ViewMode) => void] {
  const [viewMode, setViewMode] = React.useState<ViewMode>(defaultMode);

  // Load from localStorage on mount
  React.useEffect(() => {
    if (storageKey && typeof window !== 'undefined') {
      const stored = localStorage.getItem(storageKey);
      if (stored && ['grid', 'list', 'compact'].includes(stored)) {
        setViewMode(stored as ViewMode);
      }
    }
  }, [storageKey]);

  // Handler that persists to localStorage
  const handleChange = React.useCallback(
    (mode: ViewMode) => {
      setViewMode(mode);
      if (storageKey && typeof window !== 'undefined') {
        localStorage.setItem(storageKey, mode);
      }
    },
    [storageKey]
  );

  return [viewMode, handleChange];
}

// =============================================================================
// VIEW MODE WRAPPER (For conditional rendering)
// =============================================================================

interface ViewModeContentProps {
  mode: ViewMode;
  currentMode: ViewMode;
  children: React.ReactNode;
}

export function ViewModeContent({ mode, currentMode, children }: ViewModeContentProps) {
  if (mode !== currentMode) return null;
  return <>{children}</>;
}

// =============================================================================
// PROJECTS VIEW HEADER (Common pattern)
// =============================================================================

interface ProjectsViewHeaderProps {
  title?: string;
  count?: number;
  viewMode: ViewMode;
  onViewModeChange: (mode: ViewMode) => void;
  actions?: React.ReactNode;
  className?: string;
}

export function ProjectsViewHeader({
  title = 'Projects',
  count,
  viewMode,
  onViewModeChange,
  actions,
  className,
}: ProjectsViewHeaderProps) {
  return (
    <div className={cn('flex items-center justify-between gap-4', className)}>
      <div className="flex items-center gap-2">
        <h2 className="text-lg font-semibold text-foreground">{title}</h2>
        {typeof count === 'number' && (
          <span className="text-sm text-muted-foreground">({count})</span>
        )}
      </div>
      <div className="flex items-center gap-3">
        {actions}
        <ViewToggle value={viewMode} onChange={onViewModeChange} />
      </div>
    </div>
  );
}
