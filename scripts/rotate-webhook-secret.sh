#!/usr/bin/env bash
#
# rotate-webhook-secret.sh - Rotate GitHub webhook secret for Enclii
#
# This script safely rotates the GitHub webhook secret by:
# 1. Generating a new cryptographically secure secret
# 2. Updating the Kubernetes secret
# 3. Updating the GitHub webhook configuration
# 4. Restarting the API to pick up the new secret
# 5. Verifying the webhook is working
#
# Usage: ./scripts/rotate-webhook-secret.sh [--dry-run]
#
# Requirements:
# - kubectl configured with access to enclii namespace
# - gh CLI authenticated with repo admin access
# - openssl for secret generation
#

set -euo pipefail

# Configuration
NAMESPACE="${ENCLII_NAMESPACE:-enclii}"
SECRET_NAME="enclii-github-webhook"
SECRET_KEY="secret"
DEPLOYMENT_NAME="switchyard-api"
REPO="madfam-org/enclii"
WEBHOOK_ID="${ENCLII_WEBHOOK_ID:-585841923}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if running in dry-run mode
DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
    DRY_RUN=true
    log_warn "Running in DRY-RUN mode - no changes will be made"
fi

# Verify prerequisites
verify_prerequisites() {
    log_info "Verifying prerequisites..."

    local missing=()

    if ! command -v kubectl &> /dev/null; then
        missing+=("kubectl")
    fi

    if ! command -v gh &> /dev/null; then
        missing+=("gh CLI")
    fi

    if ! command -v openssl &> /dev/null; then
        missing+=("openssl")
    fi

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing[*]}"
        exit 1
    fi

    # Verify kubectl access
    if ! kubectl auth can-i get secrets -n "$NAMESPACE" &> /dev/null; then
        log_error "kubectl does not have access to secrets in namespace: $NAMESPACE"
        exit 1
    fi

    # Verify gh auth
    if ! gh auth status &> /dev/null; then
        log_error "gh CLI is not authenticated. Run: gh auth login"
        exit 1
    fi

    # Verify repo access
    if ! gh api "repos/$REPO" &> /dev/null; then
        log_error "Cannot access repository: $REPO"
        exit 1
    fi

    log_success "All prerequisites verified"
}

# Generate new secret
generate_secret() {
    log_info "Generating new webhook secret..."
    NEW_SECRET=$(openssl rand -hex 32)
    log_success "Generated new 256-bit secret"
    echo "$NEW_SECRET"
}

# Update Kubernetes secret
update_k8s_secret() {
    local secret="$1"
    log_info "Updating Kubernetes secret: $SECRET_NAME in namespace: $NAMESPACE"

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "[DRY-RUN] Would update K8s secret with new value"
        return 0
    fi

    # Create or update the secret
    kubectl create secret generic "$SECRET_NAME" \
        --from-literal="$SECRET_KEY=$secret" \
        --dry-run=client -o yaml | kubectl apply -n "$NAMESPACE" -f -

    log_success "Kubernetes secret updated"
}

# Update GitHub webhook
update_github_webhook() {
    local secret="$1"
    log_info "Updating GitHub webhook for repo: $REPO (webhook ID: $WEBHOOK_ID)"

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "[DRY-RUN] Would update GitHub webhook with new secret"
        return 0
    fi

    # Update the webhook secret
    gh api "repos/$REPO/hooks/$WEBHOOK_ID" -X PATCH \
        -f "config[secret]=$secret" \
        -f "config[content_type]=json" \
        -f "config[insecure_ssl]=0" \
        --silent

    log_success "GitHub webhook updated"
}

# Restart API deployment
restart_api() {
    log_info "Restarting API deployment to pick up new secret..."

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "[DRY-RUN] Would restart deployment: $DEPLOYMENT_NAME"
        return 0
    fi

    kubectl rollout restart "deploy/$DEPLOYMENT_NAME" -n "$NAMESPACE"

    log_info "Waiting for rollout to complete..."
    kubectl rollout status "deploy/$DEPLOYMENT_NAME" -n "$NAMESPACE" --timeout=120s

    log_success "API deployment restarted and ready"
}

# Verify webhook is working
verify_webhook() {
    log_info "Verifying webhook configuration..."

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "[DRY-RUN] Would verify webhook status"
        return 0
    fi

    # Wait a moment for the API to be fully ready
    sleep 5

    # Send a test ping to the webhook
    log_info "Sending test ping to webhook..."
    gh api "repos/$REPO/hooks/$WEBHOOK_ID/pings" -X POST --silent || true

    # Wait for the ping to be processed
    sleep 3

    # Check the last delivery status
    local last_delivery
    last_delivery=$(gh api "repos/$REPO/hooks/$WEBHOOK_ID/deliveries" --jq '.[0]')

    local status_code
    status_code=$(echo "$last_delivery" | jq -r '.status_code')
    local event
    event=$(echo "$last_delivery" | jq -r '.event')
    local delivered_at
    delivered_at=$(echo "$last_delivery" | jq -r '.delivered_at')

    log_info "Last webhook delivery: event=$event, status=$status_code, at=$delivered_at"

    if [[ "$status_code" == "200" ]]; then
        log_success "Webhook is working correctly!"
        return 0
    else
        log_warn "Webhook returned status $status_code - may need investigation"
        log_info "Check API logs: kubectl logs -n $NAMESPACE deploy/$DEPLOYMENT_NAME --tail=50"
        return 1
    fi
}

# Trigger a test build
trigger_test_build() {
    log_info "Triggering a test build by pushing an empty commit..."

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "[DRY-RUN] Would push empty commit to trigger build"
        return 0
    fi

    # Create and push empty commit
    git commit --allow-empty -m "chore: trigger build after webhook secret rotation

This empty commit verifies the webhook is working after secret rotation.
Generated by: scripts/rotate-webhook-secret.sh"

    git push origin main

    log_success "Test commit pushed - check webhook deliveries in ~30 seconds"
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "  Enclii Webhook Secret Rotation Script  "
    echo "=========================================="
    echo ""

    verify_prerequisites

    # Generate new secret
    NEW_SECRET=$(generate_secret)

    # Store the old webhook status for comparison
    log_info "Current webhook status:"
    gh api "repos/$REPO/hooks/$WEBHOOK_ID" --jq '{active: .active, last_response: .last_response}'
    echo ""

    # Update both sides atomically (as much as possible)
    update_k8s_secret "$NEW_SECRET"
    update_github_webhook "$NEW_SECRET"

    # Restart API to pick up new secret
    restart_api

    # Verify the webhook is working
    verify_webhook

    echo ""
    log_success "Webhook secret rotation complete!"
    echo ""

    # Offer to trigger a test build
    if [[ "$DRY_RUN" != "true" ]]; then
        read -p "Would you like to trigger a test build? (y/N) " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            trigger_test_build
        fi
    fi

    echo ""
    echo "Next steps:"
    echo "  1. Monitor webhook deliveries: gh api repos/$REPO/hooks/$WEBHOOK_ID/deliveries --jq '.[0:3]'"
    echo "  2. Check deployments page: https://app.enclii.dev/deployments"
    echo "  3. Verify builds are triggering on push to main"
    echo ""
}

# Run main
main "$@"
