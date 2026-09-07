'use client'

import { useCallback, useEffect, useState } from 'react'
import { providerApi } from '@/lib/provider-api'
import { Button } from '@enclii/ui-components/button'
import { RefreshCw, Server } from 'lucide-react'
import type { OperatorResponse, PorkbunRegistrarScope } from '@/types/providers'

// Porkbun API keys are scoped to one Porkbun ACCOUNT, so "is Porkbun
// configured?" has no single answer for this estate: MADFAM's global key
// operates the estate's own domains, and a client that keeps its own registrar
// account (CTM/creatumundo.mx) is reachable only through that client's own key
// pair. A page that showed one inventory would show MADFAM's and silently omit
// every client domain, so the scope is explicit and always on screen.
const GLOBAL_SCOPE = '__global__'

export default function ProvidersPorkbunPage() {
  const [domains, setDomains] = useState<OperatorResponse | null>(null)
  const [scopes, setScopes] = useState<PorkbunRegistrarScope[]>([])
  const [selected, setSelected] = useState<string>(GLOBAL_SCOPE)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // The catalog reports, per tenant, which registrar account its domains
  // resolve to and whether that account's credentials are usable.
  const loadScopes = useCallback(async () => {
    try {
      const catalog = await providerApi.catalog()
      const porkbun = catalog.providers?.find((p) => p.name === 'porkbun')
      const readiness = porkbun?.readiness as
        | { registrarScopes?: PorkbunRegistrarScope[] }
        | undefined
      setScopes(readiness?.registrarScopes ?? [])
    } catch {
      // A catalog failure must not hide the inventory below; the scope
      // selector simply falls back to the global account.
      setScopes([])
    }
  }, [])

  const loadDomains = useCallback(async (scope: string) => {
    setLoading(true)
    setError(null)
    try {
      const data = await providerApi.operation('porkbun', 'domains', {
        dry_run: true,
        scope: scope === GLOBAL_SCOPE ? undefined : { tenant: scope },
      })
      setDomains(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load Porkbun domains')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadScopes()
  }, [loadScopes])

  useEffect(() => {
    loadDomains(selected)
  }, [loadDomains, selected])

  const tenantScopes = scopes.filter((scope) => scope.account === 'tenant')
  const activeScope =
    selected === GLOBAL_SCOPE ? null : scopes.find((scope) => scope.tenant === selected)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="bg-primary/10 border-primary/20 rounded-lg border p-2">
            <Server className="text-primary size-5" />
          </div>
          <div>
            <h2 className="text-lg font-semibold">Porkbun</h2>
            <p className="text-muted-foreground text-sm">
              Registrar inventory and nameserver fallback, per registrar account
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <select
            aria-label="Registrar credential scope"
            className="bg-background h-9 rounded-md border px-2 text-sm"
            value={selected}
            onChange={(event) => setSelected(event.target.value)}
          >
            <option value={GLOBAL_SCOPE}>MADFAM account (global key)</option>
            {tenantScopes.map((scope) => (
              <option key={scope.tenant} value={scope.tenant}>
                {scope.display_name} account{scope.configured ? '' : ' — not configured'}
              </option>
            ))}
          </select>
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadDomains(selected)}
            disabled={loading}
            className="gap-2"
          >
            <RefreshCw className={`size-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {scopes.length > 0 && (
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-sm">
            <thead className="bg-muted/40 text-left">
              <tr>
                <th className="p-3 font-medium">Tenant</th>
                <th className="p-3 font-medium">Registrar account</th>
                <th className="p-3 font-medium">Domains</th>
                <th className="p-3 font-medium">Credentials</th>
              </tr>
            </thead>
            <tbody>
              {scopes.map((scope) => (
                <tr key={scope.tenant} className="border-t">
                  <td className="p-3">{scope.display_name}</td>
                  <td className="p-3">
                    {scope.account === 'tenant' ? (
                      <span title={scope.vault_path}>
                        {scope.display_name}&apos;s own account
                      </span>
                    ) : (
                      'MADFAM (global key)'
                    )}
                  </td>
                  <td className="text-muted-foreground p-3">
                    {scope.domains?.join(', ') || '—'}
                  </td>
                  <td className="p-3">
                    {scope.configured ? (
                      <span className="text-primary">configured</span>
                    ) : (
                      <span className="text-destructive" title={scope.detail}>
                        not configured
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {activeScope && !activeScope.configured && (
        <div className="border-destructive/30 bg-destructive/10 rounded-lg border p-4 text-sm">
          <p className="font-medium">
            {activeScope.display_name}&apos;s registrar credentials are not loaded.
          </p>
          {activeScope.detail && (
            <p className="text-muted-foreground mt-1">{activeScope.detail}</p>
          )}
          <p className="text-muted-foreground mt-2">
            Run <code>scripts/operator/porkbun-tenant-credentials.sh</code> with{' '}
            <code>ENCLII_TENANT={activeScope.tenant}</code>. The values go straight
            into Vault; no one has to paste them here.
          </p>
        </div>
      )}

      {error && (
        <div className="border-destructive/30 bg-destructive/10 text-destructive rounded-lg border p-4 text-sm">
          {error}
        </div>
      )}

      <pre className="bg-muted/30 max-h-[480px] overflow-auto rounded p-4 text-xs">
        {JSON.stringify(domains?.data ?? domains, null, 2)}
      </pre>
    </div>
  )
}
