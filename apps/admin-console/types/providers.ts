export type OperatorStep = {
  name: string
  status: string
  detail?: string
}

export type OperatorResponse = {
  operation_id?: string
  audit_id?: string
  operation: string
  status: string
  dry_run: boolean
  summary?: string
  data?: Record<string, unknown>
  steps?: OperatorStep[]
  warnings?: string[]
  next?: string[]
}

export type TenantBinding = {
  tenant: string
  display_name: string
  domain_suffixes: string[]
  default_sender: string
  resend_region: string
  resend_domain_status?: string
}

// Porkbun credentials are per Porkbun ACCOUNT. This is one row of the
// registrar table: which account a tenant's domains resolve to, and whether
// that account's credentials are usable. Never carries a credential value —
// vault_path is a location, not a secret.
export type PorkbunRegistrarScope = {
  tenant: string
  display_name: string
  account: 'madfam' | 'tenant'
  domains: string[]
  vault_path?: string
  configured: boolean
  detail?: string
}

export type ProviderCatalogEntry = {
  name: string
  status: string
  description?: string
  actions?: string[]
  scopes?: string[]
  readiness?: Record<string, unknown>
  tenant_bindings?: TenantBinding[]
}

export type ProviderCatalogResponse = {
  generated_at: string
  providers: ProviderCatalogEntry[]
  ops: Array<{ name: string; status: string; actions?: string[] }>
  ecosystem_tenants: Array<{
    id: string
    display_name: string
    domain_suffixes: string[]
    default_sender_address: string
    resend_region: string
  }>
}

export type ResendDomainRow = {
  id: string
  name: string
  status: string
  region?: string
  tenant: string
  records?: Array<{ record: string; name: string; type: string; value: string; status?: string }>
}
