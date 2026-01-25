# Compliance Webhooks Integration

Enclii automatically exports deployment evidence to compliance automation platforms (Vanta, Drata) for SOC 2, ISO 27001, HIPAA, and PCI-DSS audits.

## Overview

When a deployment completes, Enclii sends structured evidence to configured compliance webhooks including:

- **Deployment Details**: Service name, environment, image URI, release version
- **Source Control**: Git SHA, repository URL, commit message
- **Code Review Evidence**: PR URL, approver, approval timestamp, CI status
- **Supply Chain Security**: SBOM format, image signature verification
- **Compliance Receipt**: Cryptographic proof of the deployment approval chain

## Configuration

### Environment Variables

Set these environment variables on the Switchyard API:

```bash
# Enable compliance exports
COMPLIANCE_ENABLED=true

# Vanta webhook URL (from Vanta's custom integrations)
VANTA_WEBHOOK_URL=https://api.vanta.com/v1/webhooks/your-webhook-id

# Drata webhook URL (from Drata's API settings)
DRATA_WEBHOOK_URL=https://api.drata.com/v1/integrations/webhooks/your-webhook-id

# Retry configuration (optional)
COMPLIANCE_MAX_RETRIES=3           # Default: 3
COMPLIANCE_RETRY_DELAY=2s          # Default: 2s
```

### Kubernetes Deployment

Add to your `switchyard-api` deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: switchyard-api
spec:
  template:
    spec:
      containers:
        - name: api
          env:
            - name: COMPLIANCE_ENABLED
              value: "true"
            - name: VANTA_WEBHOOK_URL
              valueFrom:
                secretKeyRef:
                  name: compliance-secrets
                  key: vanta-webhook-url
            - name: DRATA_WEBHOOK_URL
              valueFrom:
                secretKeyRef:
                  name: compliance-secrets
                  key: drata-webhook-url
```

Create the secrets:

```bash
kubectl create secret generic compliance-secrets \
  --from-literal=vanta-webhook-url="https://api.vanta.com/v1/webhooks/..." \
  --from-literal=drata-webhook-url="https://api.drata.com/v1/integrations/..."
```

## Vanta Integration

### Webhook Payload Format

Enclii sends events in Vanta's expected format:

```json
{
  "event_type": "deployment.completed",
  "event_id": "deploy-abc123",
  "timestamp": "2026-01-25T10:30:00Z",
  "source": "enclii-switchyard",
  "source_version": "1.0",
  "resource": {
    "type": "deployment",
    "id": "deploy-abc123",
    "name": "api-service",
    "environment": "production"
  },
  "evidence": {
    "deployment_id": "deploy-abc123",
    "release_version": "v1.2.3",
    "image_uri": "ghcr.io/org/api:v1.2.3",
    "deployed_at": "2026-01-25T10:30:00Z",
    "git_sha": "abc123def456",
    "git_repo": "https://github.com/org/repo",
    "code_review": {
      "pr_url": "https://github.com/org/repo/pull/42",
      "pr_number": 42,
      "approved_by": "reviewer@company.com",
      "approved_at": "2026-01-25T10:25:00Z",
      "ci_status": "success",
      "verified": true
    },
    "sbom": {
      "format": "cyclonedx-json",
      "package_count": 156,
      "generated": true
    },
    "image_signature": "sha256:...",
    "signature_verified": true
  },
  "actor": {
    "email": "developer@company.com",
    "name": "Developer Name"
  }
}
```

### SOC 2 Controls Mapped

| Control | Description | Evidence Field |
|---------|-------------|----------------|
| CC8.1 | Monitoring for security events | All deployments tracked |
| CC7.1 | System operations/change management | `change_ticket` |
| CC7.2 | Code review before deployment | `code_review.approved_by` |
| CC8.2 | Change management with approval | `code_review.verified` |

### Vanta Setup Steps

1. Log in to Vanta dashboard
2. Navigate to **Integrations** > **Custom Integrations**
3. Click **Add Custom Integration**
4. Select **Webhook** as the integration type
5. Copy the generated webhook URL
6. Add the URL to Enclii's `VANTA_WEBHOOK_URL` environment variable
7. Deploy changes and verify webhooks arrive in Vanta

## Drata Integration

### Webhook Payload Format

Enclii sends events in Drata's expected format:

```json
{
  "event_type": "deployment",
  "event_id": "deploy-abc123",
  "timestamp": "2026-01-25T10:30:00Z",
  "integration": "enclii_switchyard",
  "entity": {
    "type": "deployment",
    "id": "deploy-abc123",
    "name": "api-service",
    "environment": "production",
    "tags": {
      "project": "my-project",
      "service": "api-service"
    }
  },
  "attributes": {
    "deployment_id": "deploy-abc123",
    "release_version": "v1.2.3",
    "image_uri": "ghcr.io/org/api:v1.2.3",
    "deployed_at": "2026-01-25T10:30:00Z",
    "status": "success",
    "repository": "https://github.com/org/repo",
    "commit_sha": "abc123def456",
    "pull_request": {
      "url": "https://github.com/org/repo/pull/42",
      "number": 42,
      "state": "merged",
      "approved_by": "reviewer@company.com",
      "approved_at": "2026-01-25T10:25:00Z",
      "ci_status": "success"
    },
    "security": {
      "image_signed": true,
      "signature_verified": true,
      "sbom_generated": true,
      "sbom_format": "cyclonedx-json"
    },
    "evidence": {
      "type": "deployment_approval",
      "compliance_receipt": "eyJ0eXAiOi..."
    }
  },
  "personnel": {
    "email": "developer@company.com",
    "name": "Developer Name"
  }
}
```

### Compliance Frameworks Mapped

| Framework | Control | Evidence |
|-----------|---------|----------|
| SOC 2 | CC8.1, CC7.1, CC7.2, CC8.2 | Deployment tracking, change management, code review |
| ISO 27001 | A.14.2.1, A.14.2.2 | Secure development, system change control |
| HIPAA | 164.312(c)(1) | Integrity controls (image signing) |
| PCI-DSS | 6.3.2 | Code review requirement |

### Drata Setup Steps

1. Log in to Drata dashboard
2. Navigate to **Settings** > **API & Integrations**
3. Click **Add Integration** > **Webhook**
4. Copy the generated webhook URL
5. Add the URL to Enclii's `DRATA_WEBHOOK_URL` environment variable
6. Deploy changes and verify webhooks arrive in Drata

## Evidence Collection

### What Enclii Collects Automatically

| Evidence Type | Source | Description |
|---------------|--------|-------------|
| Deployment ID | Switchyard | Unique deployment identifier |
| Git SHA | GitHub webhook | Commit being deployed |
| PR Approval | GitHub API | Who approved and when |
| CI Status | GitHub API | Whether CI passed |
| SBOM | Syft (build time) | Software bill of materials |
| Image Signature | Cosign | Cryptographic signature |
| Deployer Identity | OIDC token | Who initiated deployment |

### PR Approval Tracking (Provenance)

Enclii verifies PR approvals via the GitHub API before deployment:

1. When a deployment is triggered, Enclii fetches PR details
2. Verifies the PR was approved by at least one reviewer
3. Checks that CI checks passed
4. Records the approval chain in the compliance receipt
5. Exports this evidence to Vanta/Drata

### Supply Chain Security

Enclii generates security evidence during the build:

- **SBOM Generation**: Uses Syft to create CycloneDX or SPDX SBOMs
- **Image Signing**: Uses Cosign to sign container images
- **Signature Verification**: Validates signatures before deployment

## Retry Behavior

Compliance webhooks use exponential backoff for reliability:

| Attempt | Delay | Total Wait |
|---------|-------|------------|
| 1 | 0s | 0s |
| 2 | 2s | 2s |
| 3 | 4s | 6s |

- **5xx errors**: Retried up to `COMPLIANCE_MAX_RETRIES` times
- **4xx errors**: Not retried (indicates invalid payload)
- **Network errors**: Retried with backoff

## Monitoring

### Logs

Compliance export logs include:

```
INFO  Sending compliance evidence to Vanta (attempt 1/3)
INFO  ✓ Successfully sent compliance evidence to Vanta (status: 200)
```

Or on failure:

```
WARN  Webhook failed with status 500: internal server error
INFO  Retrying in 2s...
ERROR ✗ Vanta export failed: webhook returned status 500
```

### Metrics

The following Prometheus metrics are available:

```
# Compliance webhook attempts
switchyard_compliance_webhook_total{provider="vanta",status="success"} 42
switchyard_compliance_webhook_total{provider="drata",status="failed"} 1

# Webhook latency
switchyard_compliance_webhook_duration_seconds{provider="vanta"}
```

## Troubleshooting

### Webhooks Not Arriving

1. Verify `COMPLIANCE_ENABLED=true` is set
2. Check webhook URLs are correct
3. Review Switchyard API logs for errors
4. Test webhook URL manually:

```bash
curl -X POST https://your-webhook-url \
  -H "Content-Type: application/json" \
  -d '{"event_type":"test","timestamp":"2026-01-25T00:00:00Z"}'
```

### Missing PR Approval Data

1. Ensure GitHub integration is configured
2. Verify the PR was actually approved (not just merged)
3. Check GitHub API token has `repo` scope
4. Review Switchyard logs for GitHub API errors

### SBOM/Signature Missing

1. Verify Roundhouse build worker has Syft/Cosign installed
2. Check `GENERATE_SBOM=true` and `SIGN_IMAGES=true` in Roundhouse config
3. Ensure `COSIGN_KEY` is configured for image signing

## Security Considerations

- Webhook URLs should be stored as Kubernetes secrets
- Use HTTPS endpoints only (HTTP is rejected)
- Webhook payloads include sensitive data - ensure endpoints are authenticated
- Compliance receipts are cryptographically signed for integrity
