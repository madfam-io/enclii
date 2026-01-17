'use client';

import { useState } from 'react';
import { Button } from "@/components/ui/button";
import type { Template } from "@/app/(protected)/templates/page";

interface DeployTemplateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: { projectName: string; projectSlug: string }) => Promise<void>;
  template: Template | null;
}

export function DeployTemplateModal({ isOpen, onClose, onSubmit, template }: DeployTemplateModalProps) {
  const [projectName, setProjectName] = useState('');
  const [projectSlug, setProjectSlug] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen || !template) return null;

  const generateSlug = (name: string) => {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9\s-]/g, '')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '');
  };

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const name = e.target.value;
    setProjectName(name);
    if (!projectSlug || projectSlug === generateSlug(projectName)) {
      setProjectSlug(generateSlug(name));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      await onSubmit({
        projectName,
        projectSlug: projectSlug || generateSlug(projectName),
      });
      // Reset form on success
      setProjectName('');
      setProjectSlug('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to deploy template');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleClose = () => {
    setProjectName('');
    setProjectSlug('');
    setError(null);
    onClose();
  };

  return (
    <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50">
      <div className="relative top-20 mx-auto p-0 border w-full max-w-md shadow-lg rounded-lg bg-white">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b">
          <h3 className="text-lg font-semibold text-gray-900">
            Deploy Template
          </h3>
          <button
            type="button"
            onClick={handleClose}
            className="text-gray-400 hover:text-gray-500"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Template Info */}
        <div className="p-4 bg-gray-50 border-b">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-white border">
              <svg className="w-6 h-6 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
              </svg>
            </div>
            <div>
              <p className="font-medium text-gray-900">{template.name}</p>
              <p className="text-sm text-gray-500">{template.framework} &bull; {template.language}</p>
            </div>
          </div>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-4">
          {error && (
            <div className="mb-4 p-3 bg-status-error-muted border border-status-error/30 rounded-md">
              <p className="text-sm text-status-error">{error}</p>
            </div>
          )}

          <div className="space-y-4">
            <div>
              <label htmlFor="projectName" className="block text-sm font-medium text-gray-700 mb-1">
                Project Name
              </label>
              <input
                type="text"
                id="projectName"
                required
                placeholder="My Awesome Project"
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                value={projectName}
                onChange={handleNameChange}
              />
            </div>

            <div>
              <label htmlFor="projectSlug" className="block text-sm font-medium text-gray-700 mb-1">
                Project Slug
              </label>
              <input
                type="text"
                id="projectSlug"
                required
                placeholder="my-awesome-project"
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                value={projectSlug}
                onChange={(e) => setProjectSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
              />
              <p className="mt-1 text-xs text-gray-500">
                Used in URLs: app.enclii.dev/projects/{projectSlug || 'your-project'}
              </p>
            </div>
          </div>

          {/* What will be created */}
          <div className="mt-4 p-3 bg-status-info-muted rounded-md">
            <p className="text-sm font-medium text-status-info-foreground mb-2">This will create:</p>
            <ul className="text-sm text-status-info space-y-1">
              <li className="flex items-center">
                <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                A new project from the {template.name} template
              </li>
              <li className="flex items-center">
                <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                Pre-configured with {template.framework}
              </li>
              <li className="flex items-center">
                <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                Ready-to-deploy service configuration
              </li>
            </ul>
          </div>

          {/* Actions */}
          <div className="flex justify-end gap-2 mt-6">
            <Button type="button" variant="outline" onClick={handleClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting || !projectName}>
              {isSubmitting ? (
                <>
                  <svg className="w-4 h-4 mr-2 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>
                  Deploying...
                </>
              ) : (
                <>
                  <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                  Deploy Template
                </>
              )}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
