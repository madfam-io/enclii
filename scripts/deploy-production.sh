#!/bin/bash
# =============================================================================
# Enclii Production Deployment Script
# =============================================================================
#
# This script guides you through deploying Enclii to production infrastructure
# using Hetzner Cloud, Cloudflare, and Kubernetes (k3s).
#
# Prerequisites:
# - terraform >= 1.5.0
# - kubectl
# - hcloud CLI
# - cloudflared
# - jq
#
# Usage: ./scripts/deploy-production.sh [command]
# Commands: check, init, plan, apply, kubeconfig, post-deploy, status, destroy
# =============================================================================

set -euo pipefail

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
TF_DIR="$ROOT_DIR/infra/terraform"
K8S_DIR="$ROOT_DIR/infra/k8s"

# Source shared logging library
# shellcheck source=lib/logging.sh
source "$SCRIPT_DIR/lib/logging.sh"

# Alias for backward compatibility
banner() { enclii_banner "Production Deployment Script"; }

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing=()

    # Required tools
    command -v terraform >/dev/null 2>&1 || missing+=("terraform")
    command -v kubectl >/dev/null 2>&1 || missing+=("kubectl")
    command -v hcloud >/dev/null 2>&1 || missing+=("hcloud")
    command -v jq >/dev/null 2>&1 || missing+=("jq")

    # Optional but recommended
    command -v cloudflared >/dev/null 2>&1 || log_warn "cloudflared not found (optional but recommended)"

    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing[*]}"
        echo ""
        echo "Install missing tools:"
        echo "  brew install terraform kubectl hcloud jq cloudflared"
        exit 1
    fi

    # Check Terraform version
    TF_VERSION=$(terraform version -json | jq -r '.terraform_version')
    log_info "Terraform version: $TF_VERSION"

    # Check tfvars file
    if [ ! -f "$TF_DIR/terraform.tfvars" ]; then
        log_error "terraform.tfvars not found!"
        echo ""
        echo "Create it from the example:"
        echo "  cp $TF_DIR/terraform.tfvars.example $TF_DIR/terraform.tfvars"
        echo "  # Then edit with your credentials"
        exit 1
    fi

    log_success "All prerequisites met"
}

# Validate tfvars
validate_tfvars() {
    log_info "Validating terraform.tfvars..."

    cd "$TF_DIR"

    # Check for placeholder values
    if grep -q "YOUR_" terraform.tfvars; then
        log_error "terraform.tfvars contains placeholder values (YOUR_*)"
        echo ""
        echo "Please fill in all required values in terraform.tfvars"
        grep "YOUR_" terraform.tfvars
        exit 1
    fi

    # Check management_ips is not empty
    if grep -qE 'management_ips\s*=\s*\[\s*\]' terraform.tfvars; then
        log_warn "management_ips is empty - you won't be able to SSH to nodes!"
        echo "Add your IP: curl ifconfig.me"
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    log_success "terraform.tfvars validated"
}

# Initialize Terraform
tf_init() {
    log_info "Initializing Terraform..."
    cd "$TF_DIR"

    # Initialize without backend first if state bucket doesn't exist
    if ! terraform init -backend=false 2>/dev/null; then
        log_warn "Backend initialization failed, trying without backend..."
        terraform init -backend=false
    else
        terraform init
    fi

    log_success "Terraform initialized"
}

# Plan infrastructure
tf_plan() {
    log_info "Planning infrastructure changes..."
    cd "$TF_DIR"

    terraform plan -out=tfplan

    echo ""
    log_info "Review the plan above. If it looks correct, run:"
    echo "  ./scripts/deploy-production.sh apply"
}

# Apply infrastructure
tf_apply() {
    log_info "Applying infrastructure..."
    cd "$TF_DIR"

    if [ ! -f "tfplan" ]; then
        log_warn "No plan file found. Running plan first..."
        terraform plan -out=tfplan
    fi

    echo ""
    log_warn "This will create/modify cloud resources and incur costs!"
    read -p "Continue with apply? (y/N) " -n 1 -r
    echo

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Aborted"
        exit 0
    fi

    terraform apply tfplan

    log_success "Infrastructure deployed!"

    # Save outputs
    terraform output -json > outputs.json
    log_info "Outputs saved to $TF_DIR/outputs.json"

    echo ""
    log_info "Next steps:"
    echo "  1. Get kubeconfig: ./scripts/deploy-production.sh kubeconfig"
    echo "  2. Post-deployment setup: ./scripts/deploy-production.sh post-deploy"
}

# Get kubeconfig
get_kubeconfig() {
    log_info "Retrieving kubeconfig..."
    cd "$TF_DIR"

    # Get control plane IP
    CP_IP=$(terraform output -json control_plane_ips | jq -r '.public[0]')
    SSH_KEY=$(terraform output -raw ssh_private_key)

    # Save SSH key temporarily
    SSH_KEY_FILE=$(mktemp)
    echo "$SSH_KEY" > "$SSH_KEY_FILE"
    chmod 600 "$SSH_KEY_FILE"

    log_info "Waiting for k3s to be ready (this may take 2-3 minutes)..."

    # Wait for SSH to be available
    for i in {1..30}; do
        if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 -i "$SSH_KEY_FILE" root@"$CP_IP" "test -f /etc/rancher/k3s/k3s.yaml" 2>/dev/null; then
            break
        fi
        echo -n "."
        sleep 10
    done
    echo ""

    # Fetch kubeconfig
    KUBECONFIG_FILE="$ROOT_DIR/kubeconfig.yaml"
    ssh -o StrictHostKeyChecking=no -i "$SSH_KEY_FILE" root@"$CP_IP" "cat /etc/rancher/k3s/k3s.yaml" > "$KUBECONFIG_FILE"

    # Replace localhost with actual IP
    sed -i.bak "s/127.0.0.1/$CP_IP/g" "$KUBECONFIG_FILE"
    rm -f "${KUBECONFIG_FILE}.bak"

    # Cleanup
    rm -f "$SSH_KEY_FILE"

    log_success "Kubeconfig saved to $KUBECONFIG_FILE"
    echo ""
    echo "To use:"
    echo "  export KUBECONFIG=$KUBECONFIG_FILE"
    echo "  kubectl get nodes"
}

# Post-deployment setup
post_deploy() {
    log_info "Running post-deployment setup..."

    if [ ! -f "$ROOT_DIR/kubeconfig.yaml" ]; then
        log_error "kubeconfig.yaml not found. Run 'kubeconfig' command first."
        exit 1
    fi

    export KUBECONFIG="$ROOT_DIR/kubeconfig.yaml"

    # Wait for nodes to be ready
    log_info "Waiting for nodes to be ready..."
    kubectl wait --for=condition=ready nodes --all --timeout=300s

    log_success "All nodes ready"
    kubectl get nodes

    echo ""
    log_info "Deploying Cloudflare Tunnel..."
    cd "$TF_DIR"

    # Get tunnel token
    TUNNEL_TOKEN=$(terraform output -raw tunnel_token)

    # Create secret
    kubectl create namespace ingress --dry-run=client -o yaml | kubectl apply -f -
    kubectl create secret generic cloudflared-credentials \
        --from-literal=token="$TUNNEL_TOKEN" \
        --namespace ingress \
        --dry-run=client -o yaml | kubectl apply -f -

    # Deploy cloudflared
    kubectl apply -f "$K8S_DIR/base/cloudflared.yaml" 2>/dev/null || {
        log_warn "cloudflared manifest not found, creating..."
        cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cloudflared
  namespace: ingress
spec:
  replicas: 2
  selector:
    matchLabels:
      app: cloudflared
  template:
    metadata:
      labels:
        app: cloudflared
    spec:
      containers:
      - name: cloudflared
        image: cloudflare/cloudflared:latest
        args:
        - tunnel
        - --no-autoupdate
        - run
        - --token
        - \$(TUNNEL_TOKEN)
        env:
        - name: TUNNEL_TOKEN
          valueFrom:
            secretKeyRef:
              name: cloudflared-credentials
              key: token
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "500m"
EOF
    }

    log_success "Cloudflare Tunnel deployed"

    echo ""
    log_info "Creating namespaces..."
    for ns in enclii-production enclii-staging monitoring; do
        kubectl create namespace $ns --dry-run=client -o yaml | kubectl apply -f -
    done

    log_success "Namespaces created"

    echo ""
    log_info "Deploying Sealed Secrets controller..."
    kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.5/controller.yaml

    log_success "Post-deployment setup complete!"

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🎉 Enclii infrastructure is ready!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Next steps:"
    echo "  1. Deploy PostgreSQL: kubectl apply -f $K8S_DIR/base/postgres.yaml"
    echo "  2. Deploy Redis: kubectl apply -f $K8S_DIR/base/redis.yaml"
    echo "  3. Deploy Switchyard: kubectl apply -f $K8S_DIR/base/switchyard.yaml"
    echo ""
    echo "Check status: ./scripts/deploy-production.sh status"
}

# Status check
status() {
    log_info "Checking deployment status..."

    if [ -f "$ROOT_DIR/kubeconfig.yaml" ]; then
        export KUBECONFIG="$ROOT_DIR/kubeconfig.yaml"

        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📊 CLUSTER STATUS"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        kubectl get nodes -o wide

        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📦 NAMESPACES"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        kubectl get namespaces

        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "🚀 DEPLOYMENTS"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        kubectl get deployments -A

        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "🔌 SERVICES"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        kubectl get services -A
    else
        log_warn "kubeconfig.yaml not found"
    fi

    if [ -f "$TF_DIR/outputs.json" ]; then
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "🌐 INFRASTRUCTURE"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        cd "$TF_DIR"
        echo "Control Plane IPs: $(terraform output -json control_plane_ips 2>/dev/null | jq -r '.public | join(", ")')"
        echo "Worker IPs: $(terraform output -json worker_ips 2>/dev/null | jq -r '.public | join(", ")')"
        echo "Tunnel ID: $(terraform output -raw tunnel_id 2>/dev/null)"
    fi
}

# Destroy infrastructure
destroy() {
    log_warn "This will DESTROY all Enclii infrastructure!"
    log_warn "All data will be PERMANENTLY LOST!"
    echo ""
    read -p "Type 'destroy-enclii' to confirm: " confirm

    if [ "$confirm" != "destroy-enclii" ]; then
        log_info "Aborted"
        exit 0
    fi

    cd "$TF_DIR"
    terraform destroy

    log_success "Infrastructure destroyed"
}

# Main
main() {
    banner

    case "${1:-}" in
        check)
            check_prerequisites
            validate_tfvars
            ;;
        init)
            check_prerequisites
            validate_tfvars
            tf_init
            ;;
        plan)
            check_prerequisites
            tf_plan
            ;;
        apply)
            check_prerequisites
            tf_apply
            ;;
        kubeconfig)
            get_kubeconfig
            ;;
        post-deploy)
            post_deploy
            ;;
        status)
            status
            ;;
        destroy)
            destroy
            ;;
        *)
            echo "Enclii Production Deployment Script"
            echo ""
            echo "Usage: $0 <command>"
            echo ""
            echo "Commands:"
            echo "  check       - Verify prerequisites and configuration"
            echo "  init        - Initialize Terraform"
            echo "  plan        - Plan infrastructure changes"
            echo "  apply       - Apply infrastructure (creates resources)"
            echo "  kubeconfig  - Retrieve kubeconfig from cluster"
            echo "  post-deploy - Run post-deployment setup (tunnel, secrets, namespaces)"
            echo "  status      - Check deployment status"
            echo "  destroy     - Destroy all infrastructure (DANGER!)"
            echo ""
            echo "Typical workflow:"
            echo "  1. cp infra/terraform/terraform.tfvars.example infra/terraform/terraform.tfvars"
            echo "  2. Edit terraform.tfvars with your credentials"
            echo "  3. $0 check"
            echo "  4. $0 init"
            echo "  5. $0 plan"
            echo "  6. $0 apply"
            echo "  7. $0 kubeconfig"
            echo "  8. $0 post-deploy"
            echo "  9. $0 status"
            ;;
    esac
}

main "$@"
