/**
 * TypeScript types for Networking components
 * Maps to Go types in packages/sdk-go/pkg/types/types.go
 */

export interface DomainInfo {
  id: string;
  domain: string;
  environment: string;
  environment_id: string;
  is_platform_domain: boolean;
  status: DomainStatus;
  tls_status: string;
  tls_provider: string;
  zero_trust_enabled: boolean;
  dns_verified_at?: string;
  verification_txt?: string;
  dns_cname?: string;
  created_at: string;
}

export type DomainStatus = 'pending' | 'active' | 'failed' | 'verifying';
export type TLSStatus = 'pending' | 'provisioning' | 'active' | 'failed';
export type TunnelStatus = 'active' | 'degraded' | 'inactive';

export interface TunnelStatusInfo {
  tunnel_id: string;
  tunnel_name: string;
  status: TunnelStatus;
  cname: string;
  connectors: number;
  last_health_check?: string;
}

export interface InternalRoute {
  path: string;
  target_service: string;
  target_port: number;
}

export interface ServiceNetworking {
  service_id: string;
  service_name: string;
  domains: DomainInfo[];
  internal_routes: InternalRoute[];
  tunnel_status?: TunnelStatusInfo;
}

export interface Environment {
  id: string;
  name: string;
  project_id: string;
  is_production: boolean;
  created_at: string;
}

export interface AddDomainRequest {
  domain?: string;
  environment_id: string;
  is_platform_domain: boolean;
  tls_provider?: string;
  zero_trust_enabled?: boolean;
}

export interface DNSInstructions {
  verification: {
    type: string;
    name: string;
    value: string;
  };
  cname: {
    type: string;
    name: string;
    value: string;
  };
}

export interface AddDomainResponse {
  domain: DomainInfo;
  message: string;
  dns_instructions?: DNSInstructions;
}
