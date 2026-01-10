export interface Deployment {
  id: string;
  release_id: string;
  environment_id: string;
  replicas: number;
  status: 'pending' | 'deploying' | 'running' | 'failed' | 'stopped';
  health: 'healthy' | 'unhealthy' | 'unknown';
  created_at: string;
  updated_at: string;
  // Git and PR information
  git_sha?: string;
  git_branch?: string;
  pr_number?: number;
  pr_title?: string;
  pr_url?: string;
  commit_message?: string;
  commit_author?: string;
}

export interface Release {
  id: string;
  service_id: string;
  version: string;
  image_uri: string;
  git_sha: string;
  status: 'building' | 'ready' | 'failed';
  created_at: string;
  updated_at: string;
}

export interface DeploymentWithRelease extends Deployment {
  release?: Release;
}

export interface DeploymentsListResponse {
  service_id: string;
  deployments: Deployment[];
  count: number;
}

export interface RollbackResponse {
  message: string;
  rolled_back_to: Deployment;
  current_deployment: Deployment;
}

export interface ReleasesListResponse {
  releases: Release[];
}
