#!/usr/bin/env bash
# Setup ArgoCD repository credentials for OCI registries
#
# Usage:
#   ./setup-credentials.sh
#
# Prerequisites:
#   - kubectl configured with cluster access
#   - GITHUB_TOKEN environment variable set (PAT with read:packages scope)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="argocd"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check prerequisites
check_prereqs() {
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Check your kubeconfig."
        exit 1
    fi

    if [ -z "${GITHUB_TOKEN:-}" ]; then
        log_error "GITHUB_TOKEN environment variable not set."
        echo ""
        echo "Generate a GitHub PAT at: https://github.com/settings/tokens/new?scopes=read:packages,repo"
        echo "Then run: export GITHUB_TOKEN=ghp_your_token_here"
        exit 1
    fi

    if [ -z "${GITHUB_USERNAME:-}" ]; then
        log_warn "GITHUB_USERNAME not set. Defaulting to 'madfam-bot'."
        log_warn "Set it explicitly: export GITHUB_USERNAME=madfam-bot"
    fi
}

# Create OCI registry credentials
create_oci_creds() {
    log_info "Creating ghcr.io OCI registry credentials..."

    kubectl create secret generic ghcr-oci-creds \
        --namespace="${NAMESPACE}" \
        --from-literal=url=ghcr.io \
        --from-literal=type=helm \
        --from-literal=enableOCI=true \
        --from-literal=username=madfam-org \
        --from-literal=password="${GITHUB_TOKEN}" \
        --dry-run=client -o yaml | \
    kubectl label --local -f - argocd.argoproj.io/secret-type=repository -o yaml | \
    kubectl apply -f -

    log_info "OCI credentials created successfully"
}

# Create credential template for all ghcr.io repos
create_creds_template() {
    log_info "Creating ghcr.io credential template..."

    kubectl create secret generic ghcr-creds-template \
        --namespace="${NAMESPACE}" \
        --from-literal=url=https://ghcr.io \
        --from-literal=type=helm \
        --from-literal=enableOCI=true \
        --from-literal=username=madfam-org \
        --from-literal=password="${GITHUB_TOKEN}" \
        --dry-run=client -o yaml | \
    kubectl label --local -f - argocd.argoproj.io/secret-type=repo-creds -o yaml | \
    kubectl apply -f -

    log_info "Credential template created successfully"
}

# Create git repository credentials
create_git_creds() {
    log_info "Creating GitHub git repository credentials..."

    kubectl create secret generic github-repo-creds \
        --namespace="${NAMESPACE}" \
        --from-literal=url=https://github.com/madfam-org \
        --from-literal=type=git \
        --from-literal=username=madfam-org \
        --from-literal=password="${GITHUB_TOKEN}" \
        --dry-run=client -o yaml | \
    kubectl label --local -f - argocd.argoproj.io/secret-type=repo-creds -o yaml | \
    kubectl apply -f -

    log_info "Git credentials created successfully"
}

# Create Image Updater git write-back credentials
# This is the secret that argocd-image-updater reads for pushing
# image digest updates back to the git repository.
create_image_updater_git_creds() {
    log_info "Creating Image Updater git write-back credentials..."

    # Note on GitHub identity:
    # - username: Use a bot account (madfam-bot) or org owner (madfam-io)
    #   that has write access to madfam-org/enclii
    # - password: A GitHub PAT (classic: ghp_ with repo scope, or
    #   fine-grained: github_pat_ with Contents read+write on madfam-org/enclii)

    kubectl create secret generic git-creds \
        --namespace="${NAMESPACE}" \
        --from-literal=username="${GITHUB_USERNAME:-madfam-bot}" \
        --from-literal=password="${GITHUB_TOKEN}" \
        --dry-run=client -o yaml | \
    kubectl apply -f -

    log_info "Image Updater git-creds created successfully"
}

# Verify credentials
verify_creds() {
    log_info "Verifying credentials..."

    echo ""
    echo "Repository credentials in ArgoCD namespace:"
    kubectl get secrets -n "${NAMESPACE}" -l argocd.argoproj.io/secret-type=repository
    kubectl get secrets -n "${NAMESPACE}" -l argocd.argoproj.io/secret-type=repo-creds
    echo ""

    log_info "To test OCI access, sync an application:"
    echo "  kubectl patch application arc-runners -n argocd --type merge -p '{\"operation\":{\"sync\":{}}}'"
}

main() {
    echo "========================================"
    echo "ArgoCD Repository Credentials Setup"
    echo "========================================"
    echo ""

    check_prereqs

    # Ensure namespace exists
    kubectl get namespace "${NAMESPACE}" &> /dev/null || {
        log_error "ArgoCD namespace not found. Is ArgoCD installed?"
        exit 1
    }

    create_oci_creds
    create_creds_template
    create_git_creds
    create_image_updater_git_creds

    echo ""
    verify_creds

    echo ""
    log_info "Setup complete! ArgoCD can now pull from ghcr.io OCI registries."
}

main "$@"
