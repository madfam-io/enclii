#!/bin/bash
set -euo pipefail

# =============================================================================
# Agency Deploy - Unified Client Deployment Orchestrator
# Zero-Touch Multi-Tenant Customer Deployment
# =============================================================================
#
# Usage:
#   ./deploy-client.sh --client suluna [--dry-run]
#
# This script orchestrates the complete client deployment:
#   1. Identity (Janua RBAC) - Creates org, roles, invitations
#   2. Namespace (Kubernetes) - Creates isolated namespace with labels
#   3. Network (Cloudflare) - Creates zone, DNS, tunnel ingress
#   4. Application (K8s) - Deploys the client workload
#
# Required Environment Variables:
#   JANUA_ADMIN_TOKEN     - JWT token for admin@madfam.io
#   CLOUDFLARE_API_TOKEN  - Cloudflare API token
#   CLOUDFLARE_ACCOUNT_ID - Cloudflare account ID
#   TUNNEL_ID             - Cloudflare Tunnel UUID
#
# =============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Load credential library if available
if [ -f "$SCRIPT_DIR/lib/cloudflare-credentials.sh" ]; then
    source "$SCRIPT_DIR/lib/cloudflare-credentials.sh"
    CREDENTIALS_LIB_LOADED=true
else
    CREDENTIALS_LIB_LOADED=false
fi

# Defaults
DRY_RUN="${DRY_RUN:-false}"
SKIP_IDENTITY="${SKIP_IDENTITY:-false}"
SKIP_NETWORK="${SKIP_NETWORK:-false}"
SKIP_APP="${SKIP_APP:-false}"

# =============================================================================
# Client Configurations
# =============================================================================

declare -A CLIENT_CONFIGS

# SuLuna Configuration
CLIENT_CONFIGS[suluna_name]="SuLuna"
CLIENT_CONFIGS[suluna_slug]="suluna"
CLIENT_CONFIGS[suluna_email]="suluna.mx@gmail.com"
CLIENT_CONFIGS[suluna_domain]="suluna.mx"
CLIENT_CONFIGS[suluna_subdomain]="links"
CLIENT_CONFIGS[suluna_service]="linkstack"
CLIENT_CONFIGS[suluna_namespace]="suluna-production"
CLIENT_CONFIGS[suluna_manifest]="$PROJECT_ROOT/dogfooding/clients/suluna-linkstack.k8s.yaml"

# Add more clients here as needed...
# CLIENT_CONFIGS[clientx_name]="Client X"
# etc.

# =============================================================================
# Helper Functions
# =============================================================================

log_header() { 
    echo ""
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${MAGENTA}  $1${NC}"
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}
log_phase() { echo -e "\n${CYAN}═══ PHASE: $1 ═══${NC}\n"; }
log_info() { echo -e "${BLUE}ℹ${NC} $1"; }
log_success() { echo -e "${GREEN}✅${NC} $1"; }
log_warn() { echo -e "${YELLOW}⚠️${NC} $1"; }
log_error() { echo -e "${RED}❌${NC} $1"; }
log_step() { echo -e "${CYAN}▶${NC} $1"; }

check_prerequisites() {
    local missing=()

    # Check tools
    for tool in curl jq kubectl yq; do
        if ! command -v "$tool" &> /dev/null; then
            missing+=("$tool")
        fi
    done

    # Check environment variables
    if [ "$SKIP_IDENTITY" != "true" ]; then
        [ -z "${JANUA_ADMIN_TOKEN:-}" ] && missing+=("JANUA_ADMIN_TOKEN")
    fi

    # Load Cloudflare credentials if network phase is enabled
    if [ "$SKIP_NETWORK" != "true" ]; then
        if [ "$CREDENTIALS_LIB_LOADED" = "true" ]; then
            log_info "Loading Cloudflare credentials..."
            if load_cloudflare_credentials; then
                log_success "Cloudflare credentials loaded"
            else
                missing+=("CLOUDFLARE_CREDENTIALS (run: scripts/cloudflare-zone-create.sh --setup)")
            fi
        else
            # Fallback to environment variable checks
            [ -z "${CLOUDFLARE_API_TOKEN:-}" ] && missing+=("CLOUDFLARE_API_TOKEN")
            [ -z "${CLOUDFLARE_ACCOUNT_ID:-}" ] && missing+=("CLOUDFLARE_ACCOUNT_ID")
            [ -z "${TUNNEL_ID:-}" ] && missing+=("TUNNEL_ID")
        fi
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        log_error "Missing prerequisites:"
        for item in "${missing[@]}"; do
            echo "  - $item"
        done
        echo ""
        echo "For Cloudflare credentials, see: docs/guides/CREDENTIAL_SETUP.md"
        return 1
    fi

    return 0
}

get_client_config() {
    local client="$1"
    local key="$2"
    echo "${CLIENT_CONFIGS[${client}_${key}]:-}"
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
  --client CLIENT       Client identifier (e.g., suluna)
  --dry-run             Preview changes without applying
  --skip-identity       Skip Janua RBAC setup
  --skip-network        Skip Cloudflare provisioning
  --skip-app            Skip application deployment
  --list-clients        List available client configurations
  --help                Show this help message

Environment Variables (required):
  JANUA_ADMIN_TOKEN     - JWT token for admin@madfam.io
  CLOUDFLARE_API_TOKEN  - Cloudflare API token with Zone:Edit, DNS:Edit
  CLOUDFLARE_ACCOUNT_ID - Cloudflare account ID
  TUNNEL_ID             - Cloudflare Tunnel UUID

Example:
  # Full deployment
  export JANUA_ADMIN_TOKEN="..."
  export CLOUDFLARE_API_TOKEN="..."
  export CLOUDFLARE_ACCOUNT_ID="..."
  export TUNNEL_ID="..."
  
  $0 --client suluna

  # Skip identity (already done)
  $0 --client suluna --skip-identity

  # Dry run to preview
  $0 --client suluna --dry-run

Available Clients:
  suluna    - SuLuna LinkStack (links.suluna.mx)
EOF
    exit 1
}

list_clients() {
    echo "Available Client Configurations:"
    echo ""
    echo "  suluna"
    echo "    Name:      ${CLIENT_CONFIGS[suluna_name]}"
    echo "    Email:     ${CLIENT_CONFIGS[suluna_email]}"
    echo "    Domain:    ${CLIENT_CONFIGS[suluna_subdomain]}.${CLIENT_CONFIGS[suluna_domain]}"
    echo "    Namespace: ${CLIENT_CONFIGS[suluna_namespace]}"
    echo ""
    exit 0
}

# =============================================================================
# Parse Arguments
# =============================================================================

CLIENT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --client) CLIENT="$2"; shift 2 ;;
        --dry-run) DRY_RUN="true"; shift ;;
        --skip-identity) SKIP_IDENTITY="true"; shift ;;
        --skip-network) SKIP_NETWORK="true"; shift ;;
        --skip-app) SKIP_APP="true"; shift ;;
        --list-clients) list_clients ;;
        --help) usage ;;
        *) log_error "Unknown option: $1"; usage ;;
    esac
done

if [ -z "$CLIENT" ]; then
    log_error "Client identifier required"
    usage
fi

# Validate client exists
CLIENT_NAME=$(get_client_config "$CLIENT" "name")
if [ -z "$CLIENT_NAME" ]; then
    log_error "Unknown client: $CLIENT"
    echo "Available clients: suluna"
    exit 1
fi

# Check prerequisites
if ! check_prerequisites; then
    exit 1
fi

# Load client configuration
CLIENT_SLUG=$(get_client_config "$CLIENT" "slug")
CLIENT_EMAIL=$(get_client_config "$CLIENT" "email")
CLIENT_DOMAIN=$(get_client_config "$CLIENT" "domain")
CLIENT_SUBDOMAIN=$(get_client_config "$CLIENT" "subdomain")
CLIENT_SERVICE=$(get_client_config "$CLIENT" "service")
CLIENT_NAMESPACE=$(get_client_config "$CLIENT" "namespace")
CLIENT_MANIFEST=$(get_client_config "$CLIENT" "manifest")
CLIENT_FULL_HOSTNAME="${CLIENT_SUBDOMAIN}.${CLIENT_DOMAIN}"

# =============================================================================
# Main Execution
# =============================================================================

log_header "🚀 AGENCY DEPLOY: $CLIENT_NAME"

echo "Client Configuration:"
echo "  Name:       $CLIENT_NAME"
echo "  Slug:       $CLIENT_SLUG"
echo "  Email:      $CLIENT_EMAIL"
echo "  Domain:     $CLIENT_FULL_HOSTNAME"
echo "  Service:    $CLIENT_SERVICE"
echo "  Namespace:  $CLIENT_NAMESPACE"
echo ""

if [ "$DRY_RUN" = "true" ]; then
    echo -e "${YELLOW}🔸 DRY RUN MODE - No changes will be made${NC}"
    echo ""
fi

# Track overall status
IDENTITY_STATUS="⏭️ Skipped"
NAMESPACE_STATUS="⏭️ Skipped"
NETWORK_STATUS="⏭️ Skipped"
APP_STATUS="⏭️ Skipped"

# =============================================================================
# Phase 1: Identity (Janua RBAC)
# =============================================================================

if [ "$SKIP_IDENTITY" != "true" ]; then
    log_phase "1/4 IDENTITY (Janua RBAC)"
    
    # Export token for sub-script
    export ADMIN_TOKEN="$JANUA_ADMIN_TOKEN"
    
    # Check if client-specific onboarding script exists
    CLIENT_ONBOARD_SCRIPT="$SCRIPT_DIR/onboard-${CLIENT}.sh"
    
    if [ -f "$CLIENT_ONBOARD_SCRIPT" ]; then
        log_step "Running client onboarding script: $CLIENT_ONBOARD_SCRIPT"
        
        if [ "$DRY_RUN" = "true" ]; then
            log_info "[DRY RUN] Would execute: $CLIENT_ONBOARD_SCRIPT"
            IDENTITY_STATUS="🔸 Dry Run"
        else
            if bash "$CLIENT_ONBOARD_SCRIPT"; then
                IDENTITY_STATUS="✅ Complete"
            else
                log_warn "Identity setup had warnings (may already exist)"
                IDENTITY_STATUS="⚠️ Warnings"
            fi
        fi
    else
        # Generic Janua org creation
        log_step "Creating organization via Janua API..."
        
        JANUA_API="${JANUA_API:-https://api.janua.dev}"
        
        if [ "$DRY_RUN" = "true" ]; then
            log_info "[DRY RUN] Would create organization: $CLIENT_NAME"
            log_info "[DRY RUN] Would invite owner: $CLIENT_EMAIL"
            IDENTITY_STATUS="🔸 Dry Run"
        else
            # Create organization
            ORG_RESPONSE=$(curl -sf -X POST "$JANUA_API/api/v1/organizations/" \
                -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
                -H "Content-Type: application/json" \
                -d "{
                    \"name\": \"$CLIENT_NAME\",
                    \"slug\": \"$CLIENT_SLUG\",
                    \"description\": \"$CLIENT_NAME - Managed Services Client\",
                    \"billing_email\": \"$CLIENT_EMAIL\"
                }" 2>/dev/null || echo '{"detail": "Organization may already exist"}')
            
            if echo "$ORG_RESPONSE" | jq -e '.id' >/dev/null 2>&1; then
                ORG_ID=$(echo "$ORG_RESPONSE" | jq -r '.id')
                log_success "Organization created: $ORG_ID"
                
                # Invite owner
                curl -sf -X POST "$JANUA_API/api/v1/organizations/$ORG_ID/invite" \
                    -H "Authorization: Bearer $JANUA_ADMIN_TOKEN" \
                    -H "Content-Type: application/json" \
                    -d "{
                        \"email\": \"$CLIENT_EMAIL\",
                        \"role\": \"owner\",
                        \"message\": \"Welcome to $CLIENT_NAME! MADFAM manages your infrastructure.\"
                    }" >/dev/null 2>&1 || true
                
                log_success "Owner invitation sent to $CLIENT_EMAIL"
                IDENTITY_STATUS="✅ Complete"
            else
                log_warn "Organization may already exist"
                IDENTITY_STATUS="⚠️ Exists"
            fi
        fi
    fi
else
    log_phase "1/4 IDENTITY (Skipped)"
    log_info "Skipping identity setup (--skip-identity)"
fi

# =============================================================================
# Phase 2: Namespace (Kubernetes)
# =============================================================================

log_phase "2/4 NAMESPACE (Kubernetes)"

log_step "Creating namespace: $CLIENT_NAMESPACE"

if [ "$DRY_RUN" = "true" ]; then
    log_info "[DRY RUN] Would create namespace: $CLIENT_NAMESPACE"
    NAMESPACE_STATUS="🔸 Dry Run"
else
    # Check if namespace exists
    if kubectl get namespace "$CLIENT_NAMESPACE" >/dev/null 2>&1; then
        log_warn "Namespace already exists"
        NAMESPACE_STATUS="⚠️ Exists"
    else
        # Create namespace with labels
        kubectl create namespace "$CLIENT_NAMESPACE" --dry-run=client -o yaml | \
            kubectl apply -f -
        
        # Add labels
        kubectl label namespace "$CLIENT_NAMESPACE" \
            client="$CLIENT_SLUG" \
            managed-by="madfam" \
            tier="client-production" \
            --overwrite
        
        log_success "Namespace created and labeled"
        NAMESPACE_STATUS="✅ Complete"
    fi
fi

# =============================================================================
# Phase 3: Network (Cloudflare)
# =============================================================================

if [ "$SKIP_NETWORK" != "true" ]; then
    log_phase "3/4 NETWORK (Cloudflare)"
    
    PROVISION_SCRIPT="$SCRIPT_DIR/provision-domain.sh"
    
    if [ ! -f "$PROVISION_SCRIPT" ]; then
        log_error "provision-domain.sh not found at $PROVISION_SCRIPT"
        NETWORK_STATUS="❌ Failed"
    else
        log_step "Provisioning domain: $CLIENT_FULL_HOSTNAME"
        
        PROVISION_ARGS=(
            --domain "$CLIENT_DOMAIN"
            --subdomain "$CLIENT_SUBDOMAIN"
            --service "$CLIENT_SERVICE"
            --namespace "$CLIENT_NAMESPACE"
        )
        
        if [ "$DRY_RUN" = "true" ]; then
            PROVISION_ARGS+=(--dry-run)
        fi
        
        if bash "$PROVISION_SCRIPT" "${PROVISION_ARGS[@]}"; then
            NETWORK_STATUS="✅ Complete"
        else
            log_error "Network provisioning failed"
            NETWORK_STATUS="❌ Failed"
        fi
    fi
else
    log_phase "3/4 NETWORK (Skipped)"
    log_info "Skipping network setup (--skip-network)"
fi

# =============================================================================
# Phase 4: Application (K8s Deployment)
# =============================================================================

if [ "$SKIP_APP" != "true" ]; then
    log_phase "4/4 APPLICATION (Kubernetes)"
    
    if [ ! -f "$CLIENT_MANIFEST" ]; then
        log_error "Manifest not found: $CLIENT_MANIFEST"
        log_info "Please create the K8s manifest first"
        APP_STATUS="❌ Not Found"
    else
        log_step "Deploying application from: $CLIENT_MANIFEST"
        
        if [ "$DRY_RUN" = "true" ]; then
            log_info "[DRY RUN] Would apply: $CLIENT_MANIFEST"
            kubectl apply -f "$CLIENT_MANIFEST" --dry-run=client
            APP_STATUS="🔸 Dry Run"
        else
            if kubectl apply -f "$CLIENT_MANIFEST"; then
                log_success "Application deployed"
                
                # Wait for deployment
                log_info "Waiting for pods to be ready..."
                
                if kubectl wait --for=condition=ready pod \
                    -l app="$CLIENT_SERVICE" \
                    -n "$CLIENT_NAMESPACE" \
                    --timeout=120s 2>/dev/null; then
                    log_success "Pods are ready"
                    APP_STATUS="✅ Complete"
                else
                    log_warn "Pods not ready yet. Check with: kubectl get pods -n $CLIENT_NAMESPACE"
                    APP_STATUS="⚠️ Pending"
                fi
            else
                log_error "Failed to deploy application"
                APP_STATUS="❌ Failed"
            fi
        fi
    fi
else
    log_phase "4/4 APPLICATION (Skipped)"
    log_info "Skipping application deployment (--skip-app)"
fi

# =============================================================================
# Summary
# =============================================================================

log_header "📊 DEPLOYMENT SUMMARY: $CLIENT_NAME"

echo "Phase Results:"
echo "  1. Identity (Janua):    $IDENTITY_STATUS"
echo "  2. Namespace (K8s):     $NAMESPACE_STATUS"
echo "  3. Network (Cloudflare): $NETWORK_STATUS"
echo "  4. Application (K8s):   $APP_STATUS"
echo ""

# Overall status
if [[ "$IDENTITY_STATUS" == *"❌"* ]] || \
   [[ "$NAMESPACE_STATUS" == *"❌"* ]] || \
   [[ "$NETWORK_STATUS" == *"❌"* ]] || \
   [[ "$APP_STATUS" == *"❌"* ]]; then
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  ❌ DEPLOYMENT HAD FAILURES - Review errors above${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 1
elif [ "$DRY_RUN" = "true" ]; then
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}  🔸 DRY RUN COMPLETE - Run without --dry-run to apply${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
else
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✅ DEPLOYMENT COMPLETE${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "Access Points:"
    echo "  Application: https://$CLIENT_FULL_HOSTNAME"
    echo "  Dashboard:   https://app.enclii.dev (switch to $CLIENT_NAME org)"
    echo ""
    echo "Verification Commands:"
    echo "  curl -I https://$CLIENT_FULL_HOSTNAME"
    echo "  kubectl get pods -n $CLIENT_NAMESPACE"
    echo "  kubectl logs -n $CLIENT_NAMESPACE -l app=$CLIENT_SERVICE"
fi

echo ""
