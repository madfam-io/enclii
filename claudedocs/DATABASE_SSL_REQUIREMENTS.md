# Database SSL/TLS Requirements

## Current State

| Component | SSL Status | Connection String |
|-----------|------------|-------------------|
| PostgreSQL Server | `ssl=off` | N/A |
| Enclii API | `sslmode=disable` | postgres://...?sslmode=disable |
| Janua API | No sslmode (defaults to prefer) | postgres://...@95.217.198.239:5432/janua_prod |

## Risk Assessment

**Current Risk**: Medium-High
- Database traffic within cluster is unencrypted
- Credentials transmitted in plaintext
- Mitigated by: NetworkPolicy isolation, internal-only services

## Required Changes

### 1. PostgreSQL Server Configuration

Generate SSL certificates:
```bash
# Option A: Self-signed (simpler)
openssl req -new -x509 -days 365 -nodes \
  -out server.crt -keyout server.key \
  -subj "/CN=postgres.data.svc.cluster.local"

# Option B: cert-manager (production recommended)
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: postgres-tls
  namespace: data
spec:
  secretName: postgres-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - postgres.data.svc.cluster.local
EOF
```

Mount certificates and configure PostgreSQL:
```yaml
# Add to postgres StatefulSet
volumes:
  - name: postgres-tls
    secret:
      secretName: postgres-tls
      defaultMode: 0600
volumeMounts:
  - name: postgres-tls
    mountPath: /var/lib/postgresql/certs
    readOnly: true
env:
  - name: POSTGRES_SSL_CERT_FILE
    value: /var/lib/postgresql/certs/tls.crt
  - name: POSTGRES_SSL_KEY_FILE
    value: /var/lib/postgresql/certs/tls.key
```

### 2. Client Connection Updates

After SSL is enabled on the server, update all connection strings:

**Enclii (enclii-secrets)**:
```bash
kubectl patch secret -n enclii enclii-secrets --type='json' -p='[
  {"op": "replace", "path": "/data/database-url",
   "value": "'$(echo -n "postgres://enclii:PASSWORD@postgres.data.svc.cluster.local:5432/enclii?sslmode=require" | base64)'"}
]'
```

**Janua (janua-secrets)**:
```bash
kubectl patch secret -n janua janua-secrets --type='json' -p='[
  {"op": "replace", "path": "/data/database-url",
   "value": "'$(echo -n "postgresql://janua:PASSWORD@postgres:5432/janua_prod?sslmode=require" | base64)'"}
]'
```

### 3. Application Code Changes

Update default sslmode in config:
```go
// apps/switchyard-api/internal/config/config.go
viper.SetDefault("database-url", "...?sslmode=require")  // Change from disable
```

### 4. Testing

```bash
# Verify SSL connection
kubectl exec -n data postgres-0 -- psql -c "SELECT ssl_is_used();"

# Test from client pod
kubectl exec -n enclii deploy/switchyard-api -- \
  psql "$DATABASE_URL" -c "SHOW ssl_cipher;"
```

## Implementation Priority

1. **Phase 1**: Enable SSL on PostgreSQL server (infrastructure)
2. **Phase 2**: Update connection strings to `sslmode=require`
3. **Phase 3**: Update code defaults and documentation
4. **Phase 4**: Consider `sslmode=verify-full` with CA certificates

## Blockers

- Requires PostgreSQL pod restart (brief downtime)
- Certificate management strategy needs decision (self-signed vs cert-manager)
- Testing in non-production environment recommended first

## References

- [PostgreSQL SSL Documentation](https://www.postgresql.org/docs/current/ssl-tcp.html)
- [infra/SECRETS_MANAGEMENT.md](../infra/SECRETS_MANAGEMENT.md) - SSL examples
