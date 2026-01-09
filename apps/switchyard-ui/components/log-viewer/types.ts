// Log streaming types for WebSocket-based real-time logs

export interface LogMessage {
  type: 'log' | 'error' | 'info' | 'connected' | 'disconnected';
  pod?: string;
  container?: string;
  timestamp: string;
  message: string;
}

export interface LogLine {
  id: string;
  type: LogMessage['type'];
  pod?: string;
  container?: string;
  timestamp: Date;
  message: string;
}

export interface LogStreamOptions {
  serviceId?: string;
  deploymentId?: string;
  env?: string;
  lines?: number;
  timestamps?: boolean;
}

export interface LogSearchRequest {
  query: string;
  start_time?: string;
  end_time?: string;
  limit?: number;
}

export interface LogSearchResponse {
  service_id: string;
  service_name: string;
  environment: string;
  query: string;
  matches: number;
  logs: string[];
}

export interface LogHistoryResponse {
  service_id: string;
  service_name: string;
  environment: string;
  namespace: string;
  logs: string;
  lines: number;
}

export type LogLevel = 'all' | 'error' | 'warn' | 'info' | 'debug';
