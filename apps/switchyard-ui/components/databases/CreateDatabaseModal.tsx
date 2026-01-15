'use client';

import { useState } from 'react';
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { DatabaseAddonType, Project } from "@/app/(protected)/databases/page";

interface CreateDatabaseModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: {
    projectSlug: string;
    type: DatabaseAddonType;
    name: string;
    config: {
      version?: string;
      storage_gb?: number;
      memory?: string;
      replicas?: number;
    };
  }) => Promise<void>;
  projects: Project[];
}

const DATABASE_TYPES = [
  {
    value: 'postgres' as DatabaseAddonType,
    label: 'PostgreSQL',
    description: 'Powerful, open source object-relational database',
    icon: (
      <svg className="w-8 h-8 text-blue-600" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z"/>
        <path d="M12 6c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6-2.69-6-6-6zm0 10c-2.21 0-4-1.79-4-4s1.79-4 4-4 4 1.79 4 4-1.79 4-4 4z"/>
      </svg>
    ),
    versions: ['16', '15', '14', '13'],
    defaultVersion: '16',
    hasStorage: true,
    defaultStorage: 10,
    storageOptions: [5, 10, 20, 50, 100],
  },
  {
    value: 'redis' as DatabaseAddonType,
    label: 'Redis',
    description: 'In-memory data structure store, used as cache',
    icon: (
      <svg className="w-8 h-8 text-red-600" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
      </svg>
    ),
    versions: ['7', '6'],
    defaultVersion: '7',
    hasStorage: false,
    memoryOptions: ['128Mi', '256Mi', '512Mi', '1Gi', '2Gi'],
    defaultMemory: '256Mi',
  },
];

const MEMORY_OPTIONS = ['128Mi', '256Mi', '512Mi', '1Gi', '2Gi'];

export function CreateDatabaseModal({ isOpen, onClose, onSubmit, projects }: CreateDatabaseModalProps) {
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [selectedType, setSelectedType] = useState<DatabaseAddonType | null>(null);
  const [name, setName] = useState('');
  const [version, setVersion] = useState('');
  const [storageGb, setStorageGb] = useState(10);
  const [memory, setMemory] = useState('256Mi');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selectedTypeConfig = DATABASE_TYPES.find(t => t.value === selectedType);

  const handleTypeSelect = (type: DatabaseAddonType) => {
    setSelectedType(type);
    const config = DATABASE_TYPES.find(t => t.value === type);
    if (config) {
      setVersion(config.defaultVersion);
      if (config.hasStorage) {
        setStorageGb(config.defaultStorage || 10);
      }
      if (config.defaultMemory) {
        setMemory(config.defaultMemory);
      }
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!selectedProject || !selectedType || !name) {
      setError('Please fill in all required fields');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await onSubmit({
        projectSlug: selectedProject,
        type: selectedType,
        name: name.toLowerCase().replace(/[^a-z0-9-]/g, '-'),
        config: {
          version,
          storage_gb: selectedTypeConfig?.hasStorage ? storageGb : undefined,
          memory: selectedType === 'redis' ? memory : undefined,
          replicas: 1,
        },
      });
      // Reset form on success
      setSelectedProject('');
      setSelectedType(null);
      setName('');
      setVersion('');
      setStorageGb(10);
      setMemory('256Mi');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create database');
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />

      {/* Modal */}
      <div className="relative bg-white rounded-lg shadow-xl max-w-lg w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="p-6">
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-xl font-semibold">Create Database</h2>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Project Selection */}
            <div className="space-y-2">
              <Label htmlFor="project">Project</Label>
              <Select value={selectedProject} onValueChange={setSelectedProject}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a project" />
                </SelectTrigger>
                <SelectContent>
                  {projects.map((project) => (
                    <SelectItem key={project.id} value={project.slug}>
                      {project.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Database Type Selection */}
            <div className="space-y-2">
              <Label>Database Type</Label>
              <div className="grid grid-cols-2 gap-3">
                {DATABASE_TYPES.map((type) => (
                  <button
                    key={type.value}
                    type="button"
                    onClick={() => handleTypeSelect(type.value)}
                    className={`p-4 border rounded-lg text-left transition-all ${
                      selectedType === type.value
                        ? 'border-blue-500 bg-blue-50 ring-2 ring-blue-500'
                        : 'border-gray-200 hover:border-gray-300'
                    }`}
                  >
                    <div className="flex items-center gap-3 mb-2">
                      {type.icon}
                      <span className="font-medium">{type.label}</span>
                    </div>
                    <p className="text-xs text-muted-foreground">{type.description}</p>
                  </button>
                ))}
              </div>
            </div>

            {/* Name Input */}
            <div className="space-y-2">
              <Label htmlFor="name">Database Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="my-database"
                pattern="[a-z0-9-]+"
              />
              <p className="text-xs text-muted-foreground">
                Lowercase letters, numbers, and hyphens only
              </p>
            </div>

            {/* Version Selection */}
            {selectedTypeConfig && (
              <div className="space-y-2">
                <Label htmlFor="version">Version</Label>
                <Select value={version} onValueChange={setVersion}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select version" />
                  </SelectTrigger>
                  <SelectContent>
                    {selectedTypeConfig.versions.map((v) => (
                      <SelectItem key={v} value={v}>
                        {selectedTypeConfig.label} {v}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Storage Selection (PostgreSQL) */}
            {selectedTypeConfig?.hasStorage && (
              <div className="space-y-2">
                <Label>Storage Size</Label>
                <div className="flex flex-wrap gap-2">
                  {selectedTypeConfig.storageOptions?.map((size) => (
                    <button
                      key={size}
                      type="button"
                      onClick={() => setStorageGb(size)}
                      className={`px-4 py-2 border rounded-md text-sm transition-all ${
                        storageGb === size
                          ? 'border-blue-500 bg-blue-50 text-blue-700'
                          : 'border-gray-200 hover:border-gray-300'
                      }`}
                    >
                      {size} GB
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Memory Selection (Redis) */}
            {selectedType === 'redis' && (
              <div className="space-y-2">
                <Label>Memory</Label>
                <div className="flex flex-wrap gap-2">
                  {MEMORY_OPTIONS.map((mem) => (
                    <button
                      key={mem}
                      type="button"
                      onClick={() => setMemory(mem)}
                      className={`px-4 py-2 border rounded-md text-sm transition-all ${
                        memory === mem
                          ? 'border-blue-500 bg-blue-50 text-blue-700'
                          : 'border-gray-200 hover:border-gray-300'
                      }`}
                    >
                      {mem}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Actions */}
            <div className="flex items-center justify-end gap-3 pt-4 border-t">
              <Button type="button" variant="outline" onClick={onClose} disabled={isSubmitting}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting || !selectedProject || !selectedType || !name}>
                {isSubmitting ? (
                  <>
                    <svg className="w-4 h-4 mr-2 animate-spin" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                    </svg>
                    Creating...
                  </>
                ) : (
                  'Create Database'
                )}
              </Button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
