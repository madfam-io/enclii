export interface EnvironmentVariable {
  id: string;
  service_id: string;
  environment_id?: string;
  key: string;
  value?: string; // Only present when revealed
  is_secret: boolean;
  created_at: string;
  updated_at: string;
  created_by_email?: string;
}

export interface EnvironmentVariableListResponse {
  environment_variables: EnvironmentVariable[];
}

export interface CreateEnvVarRequest {
  key: string;
  value: string;
  environment_id?: string;
  is_secret?: boolean;
}

export interface UpdateEnvVarRequest {
  key?: string;
  value?: string;
  is_secret?: boolean;
}

export interface RevealedValue {
  id: string;
  key: string;
  value: string;
}
