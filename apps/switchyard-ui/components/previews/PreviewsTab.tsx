'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { apiGet, apiPost, apiDelete } from '@/lib/api';
import { PreviewEnvironment, PreviewEnvironmentListResponse, PreviewEnvironmentStatus } from './types';

interface PreviewsTabProps {
  serviceId: string;
  serviceName: string;
}

const statusConfig: Record<PreviewEnvironmentStatus, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline'; className: string }> = {
  pending: { label: 'Pending', variant: 'secondary', className: 'bg-gray-100 text-gray-800' },
  building: { label: 'Building', variant: 'default', className: 'bg-blue-100 text-blue-800 animate-pulse' },
  deploying: { label: 'Deploying', variant: 'default', className: 'bg-blue-100 text-blue-800 animate-pulse' },
  active: { label: 'Active', variant: 'default', className: 'bg-green-100 text-green-800' },
  sleeping: { label: 'Sleeping', variant: 'secondary', className: 'bg-yellow-100 text-yellow-800' },
  failed: { label: 'Failed', variant: 'destructive', className: 'bg-red-100 text-red-800' },
  closed: { label: 'Closed', variant: 'outline', className: 'bg-gray-100 text-gray-500' },
};

function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 0) return `${diffDays}d ago`;
  if (diffHours > 0) return `${diffHours}h ago`;
  if (diffMins > 0) return `${diffMins}m ago`;
  return 'Just now';
}

export function PreviewsTab({ serviceId, serviceName }: PreviewsTabProps) {
  const [previews, setPreviews] = useState<PreviewEnvironment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const fetchPreviews = async () => {
    try {
      setError(null);
      const data = await apiGet<PreviewEnvironmentListResponse>(`/v1/services/${serviceId}/previews`);
      setPreviews(data.previews || []);
    } catch (err) {
      console.error('Failed to fetch previews:', err);
      setError(err instanceof Error ? err.message : 'Failed to load preview environments');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPreviews();
    // Poll for updates every 30 seconds for active previews
    const interval = setInterval(fetchPreviews, 30000);
    return () => clearInterval(interval);
  }, [serviceId]);

  const handleWake = async (preview: PreviewEnvironment) => {
    setActionLoading(preview.id);
    try {
      await apiPost(`/v1/previews/${preview.id}/wake`, {});
      await fetchPreviews();
    } catch (err) {
      console.error('Failed to wake preview:', err);
      alert('Failed to wake preview: ' + (err instanceof Error ? err.message : 'Unknown error'));
    } finally {
      setActionLoading(null);
    }
  };

  const handleClose = async (preview: PreviewEnvironment) => {
    if (!confirm(`Close preview for PR #${preview.pr_number}? This will stop the preview deployment.`)) {
      return;
    }
    setActionLoading(preview.id);
    try {
      await apiPost(`/v1/previews/${preview.id}/close`, {});
      await fetchPreviews();
    } catch (err) {
      console.error('Failed to close preview:', err);
      alert('Failed to close preview: ' + (err instanceof Error ? err.message : 'Unknown error'));
    } finally {
      setActionLoading(null);
    }
  };

  const handleDelete = async (preview: PreviewEnvironment) => {
    if (!confirm(`Permanently delete preview for PR #${preview.pr_number}? This cannot be undone.`)) {
      return;
    }
    setActionLoading(preview.id);
    try {
      await apiDelete(`/v1/previews/${preview.id}`);
      await fetchPreviews();
    } catch (err) {
      console.error('Failed to delete preview:', err);
      alert('Failed to delete preview: ' + (err instanceof Error ? err.message : 'Unknown error'));
    } finally {
      setActionLoading(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        <span className="ml-3 text-muted-foreground">Loading preview environments...</span>
      </div>
    );
  }

  if (error) {
    return (
      <Card className="border-red-200 bg-red-50">
        <CardContent className="py-8">
          <div className="text-center">
            <p className="text-red-600 font-medium mb-4">{error}</p>
            <Button variant="outline" onClick={fetchPreviews}>
              Try Again
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  const activePreviews = previews.filter(p => !['closed', 'failed'].includes(p.status));
  const closedPreviews = previews.filter(p => ['closed', 'failed'].includes(p.status));

  return (
    <div className="space-y-6">
      {/* Header */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <svg className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M15 12H3" />
            </svg>
            Preview Environments
          </CardTitle>
          <CardDescription>
            Automatic PR-based preview deployments. Create a pull request to get a preview URL.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-4 text-sm text-muted-foreground">
            <div className="flex items-center gap-1">
              <span className="inline-block w-2 h-2 rounded-full bg-green-500"></span>
              <span>{activePreviews.filter(p => p.status === 'active').length} Active</span>
            </div>
            <div className="flex items-center gap-1">
              <span className="inline-block w-2 h-2 rounded-full bg-blue-500 animate-pulse"></span>
              <span>{activePreviews.filter(p => ['building', 'deploying', 'pending'].includes(p.status)).length} In Progress</span>
            </div>
            <div className="flex items-center gap-1">
              <span className="inline-block w-2 h-2 rounded-full bg-yellow-500"></span>
              <span>{activePreviews.filter(p => p.status === 'sleeping').length} Sleeping</span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Active Previews */}
      {activePreviews.length > 0 && (
        <div className="space-y-4">
          <h3 className="text-lg font-semibold">Active Previews</h3>
          {activePreviews.map((preview) => (
            <PreviewCard
              key={preview.id}
              preview={preview}
              onWake={handleWake}
              onClose={handleClose}
              onDelete={handleDelete}
              actionLoading={actionLoading === preview.id}
            />
          ))}
        </div>
      )}

      {/* Empty State */}
      {activePreviews.length === 0 && (
        <Card>
          <CardContent className="py-12">
            <div className="text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">No active previews</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                Preview environments are automatically created when you open a pull request.
              </p>
              <p className="mt-1 text-sm text-muted-foreground">
                Each PR gets a unique URL like <code className="bg-gray-100 px-1 rounded">pr-123-{serviceName.toLowerCase()}.preview.enclii.app</code>
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Closed Previews */}
      {closedPreviews.length > 0 && (
        <div className="space-y-4">
          <h3 className="text-lg font-semibold text-muted-foreground">Closed Previews</h3>
          {closedPreviews.slice(0, 5).map((preview) => (
            <PreviewCard
              key={preview.id}
              preview={preview}
              onWake={handleWake}
              onClose={handleClose}
              onDelete={handleDelete}
              actionLoading={actionLoading === preview.id}
              isHistorical
            />
          ))}
          {closedPreviews.length > 5 && (
            <p className="text-sm text-muted-foreground text-center">
              + {closedPreviews.length - 5} more closed previews
            </p>
          )}
        </div>
      )}
    </div>
  );
}

interface PreviewCardProps {
  preview: PreviewEnvironment;
  onWake: (preview: PreviewEnvironment) => void;
  onClose: (preview: PreviewEnvironment) => void;
  onDelete: (preview: PreviewEnvironment) => void;
  actionLoading: boolean;
  isHistorical?: boolean;
}

function PreviewCard({ preview, onWake, onClose, onDelete, actionLoading, isHistorical }: PreviewCardProps) {
  const config = statusConfig[preview.status];

  return (
    <Card className={isHistorical ? 'opacity-60' : ''}>
      <CardContent className="py-4">
        <div className="flex items-start justify-between">
          <div className="flex-1 min-w-0">
            {/* PR Info */}
            <div className="flex items-center gap-3">
              <Badge className={config.className}>
                {config.label}
              </Badge>
              <a
                href={preview.pr_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-lg font-medium hover:underline truncate"
              >
                PR #{preview.pr_number}: {preview.pr_title || 'Untitled'}
              </a>
            </div>

            {/* Branch and Commit */}
            <div className="mt-2 flex items-center gap-4 text-sm text-muted-foreground">
              <span className="flex items-center gap-1">
                <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <line x1="6" y1="3" x2="6" y2="15" />
                  <circle cx="18" cy="6" r="3" />
                  <circle cx="6" cy="18" r="3" />
                  <path d="M18 9a9 9 0 0 1-9 9" />
                </svg>
                {preview.pr_branch}
              </span>
              <span className="flex items-center gap-1">
                <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="4" />
                  <line x1="1.05" y1="12" x2="7" y2="12" />
                  <line x1="17.01" y1="12" x2="22.96" y2="12" />
                </svg>
                {preview.commit_sha.substring(0, 7)}
              </span>
              {preview.pr_author && (
                <span className="flex items-center gap-1">
                  <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                    <circle cx="12" cy="7" r="4" />
                  </svg>
                  {preview.pr_author}
                </span>
              )}
              <span>{formatTimeAgo(preview.created_at)}</span>
            </div>

            {/* Preview URL */}
            {preview.status === 'active' && (
              <div className="mt-3">
                <a
                  href={preview.preview_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 text-sm text-blue-600 hover:text-blue-800"
                >
                  <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                    <polyline points="15 3 21 3 21 9" />
                    <line x1="10" y1="14" x2="21" y2="3" />
                  </svg>
                  {preview.preview_url}
                </a>
              </div>
            )}

            {/* Status Message */}
            {preview.status_message && ['building', 'deploying', 'failed'].includes(preview.status) && (
              <p className="mt-2 text-sm text-muted-foreground">
                {preview.status_message}
              </p>
            )}

            {/* Sleeping Info */}
            {preview.status === 'sleeping' && preview.sleeping_since && (
              <p className="mt-2 text-sm text-yellow-600">
                Sleeping since {formatTimeAgo(preview.sleeping_since)} (auto-sleep after {preview.auto_sleep_after} minutes of inactivity)
              </p>
            )}
          </div>

          {/* Actions */}
          <div className="flex items-center gap-2 ml-4">
            {preview.status === 'sleeping' && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => onWake(preview)}
                disabled={actionLoading}
              >
                {actionLoading ? 'Waking...' : 'Wake Up'}
              </Button>
            )}
            {preview.status === 'active' && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => window.open(preview.preview_url, '_blank')}
              >
                Open
              </Button>
            )}
            {!['closed', 'failed'].includes(preview.status) && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => onClose(preview)}
                disabled={actionLoading}
              >
                Close
              </Button>
            )}
            {['closed', 'failed'].includes(preview.status) && (
              <Button
                size="sm"
                variant="ghost"
                className="text-red-600 hover:text-red-700"
                onClick={() => onDelete(preview)}
                disabled={actionLoading}
              >
                Delete
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
