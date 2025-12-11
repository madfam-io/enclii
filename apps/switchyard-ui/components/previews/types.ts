// Preview Environment types matching the API

export type PreviewEnvironmentStatus =
  | 'pending'
  | 'building'
  | 'deploying'
  | 'active'
  | 'sleeping'
  | 'failed'
  | 'closed';

export interface PreviewEnvironment {
  id: string;
  project_id: string;
  service_id: string;
  pr_number: number;
  pr_title?: string;
  pr_url?: string;
  pr_author?: string;
  pr_branch: string;
  pr_base_branch: string;
  commit_sha: string;
  preview_subdomain: string;
  preview_url: string;
  status: PreviewEnvironmentStatus;
  status_message?: string;
  auto_sleep_after: number;
  last_accessed_at?: string;
  sleeping_since?: string;
  deployment_id?: string;
  build_logs_url?: string;
  created_at: string;
  updated_at: string;
  closed_at?: string;
}

export interface PreviewComment {
  id: string;
  preview_environment_id: string;
  user_id?: string;
  user_email: string;
  comment: string;
  resolved: boolean;
  resolved_at?: string;
  resolved_by?: string;
  page_url?: string;
  element_selector?: string;
  created_at: string;
  updated_at: string;
}

export interface PreviewEnvironmentListResponse {
  previews: PreviewEnvironment[];
}

export interface PreviewCommentListResponse {
  comments: PreviewComment[];
}
