'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';

interface Project {
  id: string;
  name: string;
  slug: string;
  description: string;
  created_at: string;
  updated_at: string;
}

interface Service {
  id: string;
  name: string;
  project_id: string;
  git_repo: string;
  status: string;
  health: string;
  last_deployment: string;
}

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [services, setServices] = useState<{ [key: string]: Service[] }>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newProject, setNewProject] = useState({
    name: '',
    slug: '',
    description: ''
  });

  const fetchProjects = async () => {
    try {
      const response = await fetch('/api/v1/projects', {
        headers: {
          'Authorization': 'Bearer your-token-here', // TODO: Implement proper auth
        },
      });
      
      if (!response.ok) {
        throw new Error('Failed to fetch projects');
      }
      
      const data = await response.json();
      setProjects(data.projects || []);
      
      // Fetch services for each project
      const servicesData: { [key: string]: Service[] } = {};
      for (const project of data.projects || []) {
        try {
          const servicesResponse = await fetch(`/api/v1/projects/${project.slug}/services`, {
            headers: {
              'Authorization': 'Bearer your-token-here',
            },
          });
          
          if (servicesResponse.ok) {
            const servicesResult = await servicesResponse.json();
            servicesData[project.id] = servicesResult.services || [];
          }
        } catch (err) {
          console.error(`Failed to fetch services for project ${project.slug}:`, err);
        }
      }
      
      setServices(servicesData);
      setLoading(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
      setLoading(false);
    }
  };

  const createProject = async (e: React.FormEvent) => {
    e.preventDefault();
    
    try {
      const response = await fetch('/api/v1/projects', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer your-token-here',
        },
        body: JSON.stringify(newProject),
      });
      
      if (!response.ok) {
        throw new Error('Failed to create project');
      }
      
      setNewProject({ name: '', slug: '', description: '' });
      setShowCreateForm(false);
      fetchProjects(); // Refresh the list
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    }
  };

  useEffect(() => {
    fetchProjects();
  }, []);

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div className="animate-pulse">
          <div className="h-8 bg-gray-200 rounded w-1/4 mb-6"></div>
          <div className="space-y-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-24 bg-gray-200 rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div className="bg-red-50 border border-red-200 rounded-md p-4">
          <div className="flex">
            <div className="text-red-800">
              <h3 className="text-sm font-medium">Error loading projects</h3>
              <div className="mt-2 text-sm">{error}</div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
      <div className="px-4 py-6 sm:px-0">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Projects</h1>
          <button
            onClick={() => setShowCreateForm(true)}
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-enclii-blue hover:bg-enclii-blue-dark focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-enclii-blue"
          >
            Create Project
          </button>
        </div>

        {/* Create Project Modal */}
        {showCreateForm && (
          <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50">
            <div className="relative top-20 mx-auto p-5 border w-96 shadow-lg rounded-md bg-white">
              <div className="mt-3">
                <h3 className="text-lg font-medium text-gray-900 mb-4">Create New Project</h3>
                <form onSubmit={createProject}>
                  <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Project Name
                    </label>
                    <input
                      type="text"
                      required
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-enclii-blue"
                      value={newProject.name}
                      onChange={(e) => setNewProject({ ...newProject, name: e.target.value })}
                    />
                  </div>
                  <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Slug
                    </label>
                    <input
                      type="text"
                      required
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-enclii-blue"
                      value={newProject.slug}
                      onChange={(e) => setNewProject({ ...newProject, slug: e.target.value })}
                    />
                  </div>
                  <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Description
                    </label>
                    <textarea
                      className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-enclii-blue"
                      rows={3}
                      value={newProject.description}
                      onChange={(e) => setNewProject({ ...newProject, description: e.target.value })}
                    />
                  </div>
                  <div className="flex justify-end space-x-2">
                    <button
                      type="button"
                      onClick={() => setShowCreateForm(false)}
                      className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-200 rounded-md hover:bg-gray-300"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      className="px-4 py-2 text-sm font-medium text-white bg-enclii-blue rounded-md hover:bg-enclii-blue-dark"
                    >
                      Create
                    </button>
                  </div>
                </form>
              </div>
            </div>
          </div>
        )}

        {/* Projects List */}
        <div className="space-y-6">
          {projects.length === 0 ? (
            <div className="text-center py-12">
              <div className="text-gray-500 mb-4">
                <svg className="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
              </div>
              <h3 className="text-lg font-medium text-gray-900">No projects found</h3>
              <p className="text-gray-500 mt-1">Get started by creating your first project.</p>
            </div>
          ) : (
            projects.map((project) => (
              <div key={project.id} className="bg-white shadow overflow-hidden sm:rounded-lg">
                <div className="px-4 py-5 sm:p-6">
                  <div className="flex items-center justify-between">
                    <div className="flex-1">
                      <Link 
                        href={`/projects/${project.slug}`}
                        className="block hover:bg-gray-50 transition-colors duration-150 -m-2 p-2 rounded"
                      >
                        <h3 className="text-lg font-medium text-gray-900 hover:text-enclii-blue">
                          {project.name}
                        </h3>
                        <p className="text-sm text-gray-500 mt-1">{project.description}</p>
                        <div className="flex items-center mt-2 space-x-4 text-xs text-gray-400">
                          <span>Slug: {project.slug}</span>
                          <span>Created: {new Date(project.created_at).toLocaleDateString()}</span>
                        </div>
                      </Link>
                    </div>
                    <div className="flex-shrink-0 ml-4">
                      <div className="text-right">
                        <div className="text-sm font-medium text-gray-900">
                          {services[project.id]?.length || 0} services
                        </div>
                        <div className="flex space-x-1 mt-1">
                          {services[project.id]?.slice(0, 3).map((service) => (
                            <span
                              key={service.id}
                              className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                                service.health === 'healthy' 
                                  ? 'bg-green-100 text-green-800'
                                  : service.health === 'unhealthy'
                                  ? 'bg-red-100 text-red-800'
                                  : 'bg-gray-100 text-gray-800'
                              }`}
                            >
                              {service.name}
                            </span>
                          ))}
                          {services[project.id]?.length > 3 && (
                            <span className="text-xs text-gray-500">
                              +{services[project.id].length - 3} more
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}