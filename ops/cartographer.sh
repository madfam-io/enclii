#!/bin/bash
# ops/cartographer.sh - Enclii Service Discovery & Adoption Tool
# Usage: ./ops/cartographer.sh [--dry-run]
#
# Discovers K8s services across all namespaces and populates the Enclii
# services table with appropriate project mappings.

set -eo pipefail

# Configuration
DRY_RUN="${1:-}"
SSH_HOST="solarpunk@ssh.madfam.io"

# Helper function for namespace to project mapping
get_ns_project() {
  local ns="$1"
  case "$ns" in
    janua) echo "janua" ;;
    enclii) echo "enclii" ;;
    dhanam) echo "dhanam" ;;
    *) echo "solarpunk-foundry" ;;
  esac
}

# Helper function for service git repo lookup
get_service_repo() {
  local name="$1"
  local ns="$2"
  case "$name" in
    janua-api|janua-dashboard|janua-admin|janua-website|janua-docs)
      echo "https://github.com/madfam-org/janua" ;;
    switchyard-api|switchyard-ui|dispatch)
      echo "https://github.com/madfam-org/enclii" ;;
    dhanam-api|dhanam-web)
      echo "https://github.com/madfam-org/dhanam" ;;
    postgres|postgres-headless)
      echo "helm://bitnami/postgresql" ;;
    redis|redis-master|redis-headless)
      echo "helm://bitnami/redis" ;;
    prometheus|prometheus-server)
      echo "helm://prometheus-community/prometheus" ;;
    grafana)
      echo "helm://grafana/grafana" ;;
    cloudflared)
      echo "external://cloudflare/cloudflared" ;;
    cert-manager|cert-manager-webhook)
      echo "helm://jetstack/cert-manager" ;;
    argocd-server|argocd-repo-server|argocd-redis|argocd-applicationset-controller|argocd-dex-server|argocd-notifications-controller)
      echo "helm://argo/argocd" ;;
    longhorn-backend|longhorn-frontend|longhorn-admission-webhook|longhorn-conversion-webhook|longhorn-recovery-backend)
      echo "helm://longhorn/longhorn" ;;
    *)
      echo "k8s://$ns/$name" ;;
  esac
}

# Helper function for app path lookup
get_app_path() {
  local name="$1"
  case "$name" in
    janua-api) echo "apps/api" ;;
    janua-dashboard) echo "apps/dashboard" ;;
    janua-admin) echo "apps/admin" ;;
    janua-website) echo "apps/landing" ;;
    janua-docs) echo "apps/docs" ;;
    switchyard-api) echo "apps/switchyard-api" ;;
    switchyard-ui) echo "apps/switchyard-ui" ;;
    dispatch) echo "apps/dispatch" ;;
    dhanam-api) echo "apps/api" ;;
    dhanam-web) echo "apps/web" ;;
    *) echo "" ;;
  esac
}

echo "=== Cartographer: Enclii Service Discovery ==="
echo "Mode: ${DRY_RUN:-LIVE}"
echo ""

# Get all project IDs upfront
echo "Fetching project IDs..."
PROJECT_DATA=$(ssh "$SSH_HOST" "sudo kubectl exec -n data postgres-0 -- psql -U enclii -d enclii -t -c 'SELECT slug, id FROM projects;'")

JANUA_ID=$(echo "$PROJECT_DATA" | grep "janua" | head -1 | sed 's/.*| *//' | xargs)
ENCLII_ID=$(echo "$PROJECT_DATA" | grep " enclii" | head -1 | sed 's/.*| *//' | xargs)
DHANAM_ID=$(echo "$PROJECT_DATA" | grep "dhanam" | head -1 | sed 's/.*| *//' | xargs)
FOUNDRY_ID=$(echo "$PROJECT_DATA" | grep "solarpunk-foundry" | head -1 | sed 's/.*| *//' | xargs)

echo "  janua -> $JANUA_ID"
echo "  enclii -> $ENCLII_ID"
echo "  dhanam -> $DHANAM_ID"
echo "  solarpunk-foundry -> $FOUNDRY_ID"

get_project_id() {
  local slug="$1"
  case "$slug" in
    janua) echo "$JANUA_ID" ;;
    enclii) echo "$ENCLII_ID" ;;
    dhanam) echo "$DHANAM_ID" ;;
    solarpunk-foundry) echo "$FOUNDRY_ID" ;;
  esac
}

echo ""
echo "Fetching K8s services..."

# Get all K8s Services
SERVICES_JSON=$(ssh "$SSH_HOST" "sudo kubectl get services -A -o json")

echo "Generating SQL..."
echo ""

# Generate SQL file
SQL_FILE="/tmp/cartographer_$$.sql"
> "$SQL_FILE"

count=0
echo "$SERVICES_JSON" | jq -r '.items[] | "\(.metadata.namespace)|\(.metadata.name)"' | while IFS='|' read -r namespace name; do
  [[ -z "$namespace" || -z "$name" ]] && continue

  case "$namespace" in
    kube-system|kube-public|kube-node-lease|default) continue ;;
  esac

  [[ "$name" == "kubernetes" ]] && continue

  project_slug=$(get_ns_project "$namespace")
  project_id=$(get_project_id "$project_slug")
  [[ -z "$project_id" ]] && continue

  git_repo=$(get_service_repo "$name" "$namespace")
  app_path=$(get_app_path "$name")

  echo "[$namespace] $name -> $project_slug"

  if [[ -n "$app_path" ]]; then
    echo "INSERT INTO services (id, project_id, name, git_repo, app_path, created_at, updated_at) VALUES (gen_random_uuid(), '$project_id', '$name', '$git_repo', '$app_path', NOW(), NOW()) ON CONFLICT (project_id, name) DO UPDATE SET git_repo = EXCLUDED.git_repo, app_path = EXCLUDED.app_path, updated_at = NOW();" >> "$SQL_FILE"
  else
    echo "INSERT INTO services (id, project_id, name, git_repo, created_at, updated_at) VALUES (gen_random_uuid(), '$project_id', '$name', '$git_repo', NOW(), NOW()) ON CONFLICT (project_id, name) DO UPDATE SET git_repo = EXCLUDED.git_repo, updated_at = NOW();" >> "$SQL_FILE"
  fi
done

SERVICE_COUNT=$(wc -l < "$SQL_FILE" | xargs)
echo ""
echo "Generated $SERVICE_COUNT SQL statements"

if [[ "$DRY_RUN" == "--dry-run" ]]; then
  echo ""
  echo "=== DRY-RUN Complete ==="
  echo "Would insert/update $SERVICE_COUNT services"
  echo ""
  echo "Sample SQL:"
  head -3 "$SQL_FILE"
  rm -f "$SQL_FILE"
  exit 0
fi

echo ""
echo "Executing batch insert via stdin..."

# Execute SQL by piping through SSH to kubectl exec
if cat "$SQL_FILE" | ssh "$SSH_HOST" "sudo kubectl exec -i -n data postgres-0 -- psql -U enclii -d enclii"; then
  echo ""
  echo "=== Discovery Complete ==="
  echo "Successfully upserted $SERVICE_COUNT services"
else
  echo ""
  echo "ERROR: Batch insert failed"
  exit 1
fi

rm -f "$SQL_FILE"
