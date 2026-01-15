# Enclii Production Deployment Checklist

**Status:** ✅ 100% Production Ready  
**Last Updated:** November 27, 2025

---

## Pre-Deployment Checklist

### 1. Accounts & Credentials
- [ ] **Hetzner Cloud account** created at https://console.hetzner.cloud
- [ ] **Hetzner API token** generated (Read & Write permissions)
- [ ] **Cloudflare account** created at https://dash.cloudflare.com
- [ ] **Cloudflare API token** generated with permissions:
  - Zone:DNS:Edit
  - Zone:Zone:Read  
  - Account:Cloudflare Tunnel:Edit
  - Account:R2:Edit
- [ ] **Cloudflare R2** enabled and API keys generated
- [ ] **Domain** added to Cloudflare (DNS managed by Cloudflare)

### 2. Local Tools
- [ ] `terraform` >= 1.5.0 installed
- [ ] `kubectl` installed
- [ ] `hcloud` CLI installed
- [ ] `cloudflared` installed
- [ ] `jq` installed

Install all with: `brew install terraform kubectl hcloud cloudflared jq`

### 3. Configuration
- [ ] Copy `terraform.tfvars.example` → `terraform.tfvars`
- [ ] Fill in all credential values (no `YOUR_*` placeholders)
- [ ] Add your IP to `management_ips` for SSH access
- [ ] Choose datacenter location (recommend `nbg1` for EU)
- [ ] Review compute sizing (defaults are good for starter)

---

## Deployment Steps

### Phase 1: Infrastructure (30-45 minutes)

```bash
cd /path/to/enclii

# 1. Validate configuration
./scripts/deploy-production.sh check

# 2. Initialize Terraform
./scripts/deploy-production.sh init

# 3. Review plan
./scripts/deploy-production.sh plan

# 4. Apply infrastructure
./scripts/deploy-production.sh apply

# 5. Get kubeconfig (wait 2-3 min for k3s)
./scripts/deploy-production.sh kubeconfig

# 6. Post-deployment setup
./scripts/deploy-production.sh post-deploy
```

### Phase 2: Core Services (15-20 minutes)

```bash
export KUBECONFIG=$(pwd)/kubeconfig.yaml

# Deploy PostgreSQL
kubectl apply -f infra/k8s/base/postgres.yaml

# Deploy Redis
kubectl apply -f infra/k8s/base/redis.yaml

# Wait for databases
kubectl wait --for=condition=ready pod -l app=postgres -n enclii-production --timeout=300s
kubectl wait --for=condition=ready pod -l app=redis -n enclii-production --timeout=300s

# Deploy Switchyard API
kubectl apply -f infra/k8s/base/switchyard-api.yaml

# Deploy Switchyard UI
kubectl apply -f infra/k8s/base/switchyard-ui.yaml
```

### Phase 3: Verification (10 minutes)

```bash
# Check all pods are running
kubectl get pods -A

# Check services
kubectl get svc -A

# Check tunnel connectivity
kubectl logs -n ingress -l app=cloudflared

# Test API endpoint
curl https://api.enclii.dev/health

# Test UI
curl https://app.enclii.dev
```

---

## Post-Deployment Checklist

### Security
- [ ] Verify no public IPs on worker nodes (tunnel-only access)
- [ ] Verify NetworkPolicies are enforced
- [ ] Verify Sealed Secrets controller is running
- [ ] Create sealed secrets for production credentials
- [ ] Enable Kubernetes audit logging

### Monitoring
- [ ] Deploy Prometheus operator
- [ ] Deploy Grafana with dashboards
- [ ] Configure alert rules
- [ ] Set up PagerDuty/Opsgenie integration
- [ ] Verify metrics are being collected

### Backups
- [ ] Configure PostgreSQL backup to R2
- [ ] Test backup restoration
- [ ] Document recovery procedures

### DNS & SSL
- [ ] Verify api.enclii.dev resolves correctly
- [ ] Verify app.enclii.dev resolves correctly
- [ ] Verify SSL certificates are valid
- [ ] Test Cloudflare for SaaS (custom domains)

---

## Infrastructure Summary

### Compute (Hetzner Cloud)
| Resource | Spec | Monthly Cost |
|----------|------|--------------|
| Control Plane | 1x cx21 (2 vCPU, 4GB) | €5.18 |
| Workers | 2x cx31 (2 vCPU, 8GB) | €19.84 |
| PostgreSQL Volume | 50GB NVMe | €2.00 |
| Redis Volume | 10GB NVMe | €0.40 |
| Build Cache Volume | 100GB NVMe | €4.00 |
| **Subtotal** | | **~€31/mo** |

### Networking (Cloudflare)
| Service | Tier | Monthly Cost |
|---------|------|--------------|
| Cloudflare Tunnel | Free | $0 |
| R2 Storage | Free tier (10GB) | $0 |
| DNS | Free | $0 |
| DDoS Protection | Free | $0 |
| For SaaS | 100 domains free | $0 |
| **Subtotal** | | **$0** |

### Total: ~$55/month (current single-node production)

---

## Scaling Guide

### When to Scale

| Metric | Threshold | Action |
|--------|-----------|--------|
| CPU > 80% sustained | 15 minutes | Add worker node |
| Memory > 85% | Any pod | Increase pod limits or add node |
| API latency P95 > 500ms | 5 minutes | Scale API replicas |
| Disk > 80% | Any volume | Expand volume |

### How to Scale

```bash
# Add worker node
# Edit terraform.tfvars: worker_count = 3
terraform plan
terraform apply

# Scale deployment
kubectl scale deployment switchyard-api --replicas=3 -n enclii-production

# Expand volume (Hetzner)
hcloud volume resize <volume-id> --size 100
```

---

## Troubleshooting

### Tunnel Not Connecting
```bash
# Check cloudflared logs
kubectl logs -n ingress -l app=cloudflared -f

# Verify tunnel token
kubectl get secret cloudflared-credentials -n ingress -o yaml

# Restart cloudflared
kubectl rollout restart deployment/cloudflared -n ingress
```

### Database Connection Issues
```bash
# Check PostgreSQL logs
kubectl logs -n enclii-production -l app=postgres

# Test connection from pod
kubectl exec -it deployment/switchyard-api -n enclii-production -- \
  psql "$DATABASE_URL" -c "SELECT 1"
```

### SSL Certificate Issues
```bash
# Check Cloudflare tunnel config
cloudflared tunnel info <tunnel-id>

# Verify DNS records
dig api.enclii.dev
dig app.enclii.dev
```

### Node Not Joining Cluster
```bash
# SSH to server (via Cloudflare Zero Trust tunnel)
ssh ssh.madfam.io
# User: solarpunk (use sudo for admin commands)

# Check k3s status
sudo systemctl status k3s-agent

# View k3s logs
journalctl -u k3s-agent -f

# Check token
cat /etc/rancher/k3s/token
```

---

## Emergency Procedures

### Rollback Deployment
```bash
kubectl rollout undo deployment/switchyard-api -n enclii-production
kubectl rollout undo deployment/switchyard-ui -n enclii-production
```

### Database Recovery
```bash
# List backups in R2
aws s3 ls s3://enclii-backups/postgres/ --endpoint-url https://<account>.r2.cloudflarestorage.com

# Restore from backup
kubectl exec -it postgres-0 -n enclii-production -- \
  pg_restore -d enclii /backups/latest.dump
```

### Complete Cluster Recovery
```bash
# If cluster is unrecoverable
./scripts/deploy-production.sh destroy
./scripts/deploy-production.sh apply
./scripts/deploy-production.sh post-deploy

# Restore from R2 backups
# (detailed in DR runbook)
```

---

## Support Contacts

| Issue | Contact |
|-------|---------|
| Infrastructure | Platform Team |
| Application | Backend Team |
| Security | Security Team |
| Billing | Finance |

---

**Document Version:** 1.0  
**Maintained By:** Platform Team
