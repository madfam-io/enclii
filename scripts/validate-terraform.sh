#!/bin/bash
# Enclii Terraform Validation Script
# Run this to validate infrastructure before deployment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="$SCRIPT_DIR/../infra/terraform"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check Terraform
    if ! command -v terraform &> /dev/null; then
        log_error "Terraform not installed. Install from: https://terraform.io/downloads"
        exit 1
    fi

    TF_VERSION=$(terraform version -json | jq -r '.terraform_version')
    log_info "Terraform version: $TF_VERSION"

    # Check jq
    if ! command -v jq &> /dev/null; then
        log_warn "jq not installed - some features may not work"
    fi

    log_success "Prerequisites check passed"
}

check_tfvars() {
    log_info "Checking terraform.tfvars..."

    if [ ! -f "$TERRAFORM_DIR/terraform.tfvars" ]; then
        log_warn "terraform.tfvars not found"
        log_info "Creating from example..."

        if [ -f "$TERRAFORM_DIR/terraform.tfvars.example" ]; then
            cp "$TERRAFORM_DIR/terraform.tfvars.example" "$TERRAFORM_DIR/terraform.tfvars"
            log_warn "Created terraform.tfvars from example - EDIT WITH REAL VALUES!"
            log_info "Required values to set:"
            echo "  - hetzner_token"
            echo "  - cloudflare_api_token"
            echo "  - cloudflare_account_id"
            echo "  - r2_access_key_id"
            echo "  - r2_secret_access_key"
            echo "  - management_ips (your IP addresses)"
            return 1
        else
            log_error "terraform.tfvars.example not found"
            exit 1
        fi
    fi

    # Check for placeholder values
    if grep -q "your-" "$TERRAFORM_DIR/terraform.tfvars"; then
        log_warn "terraform.tfvars contains placeholder values - update before deployment"
        return 1
    fi

    log_success "terraform.tfvars exists"
    return 0
}

terraform_init() {
    log_info "Running terraform init..."
    cd "$TERRAFORM_DIR"

    if terraform init -backend=false; then
        log_success "Terraform initialized successfully"
    else
        log_error "Terraform init failed"
        exit 1
    fi
}

terraform_validate() {
    log_info "Running terraform validate..."
    cd "$TERRAFORM_DIR"

    if terraform validate; then
        log_success "Terraform configuration is valid"
    else
        log_error "Terraform validation failed"
        exit 1
    fi
}

terraform_plan() {
    log_info "Running terraform plan..."
    cd "$TERRAFORM_DIR"

    # Check if tfvars has real values
    if grep -q "your-" "$TERRAFORM_DIR/terraform.tfvars" 2>/dev/null; then
        log_warn "Skipping plan - terraform.tfvars has placeholder values"
        log_info "Update terraform.tfvars with real credentials, then run:"
        echo "  cd $TERRAFORM_DIR && terraform plan"
        return
    fi

    if terraform plan -out=tfplan; then
        log_success "Terraform plan generated: tfplan"
        log_info "Review the plan, then apply with:"
        echo "  cd $TERRAFORM_DIR && terraform apply tfplan"
    else
        log_error "Terraform plan failed"
        exit 1
    fi
}

show_cost_estimate() {
    log_info "Estimated Monthly Costs (from terraform.tfvars.example):"
    echo ""
    echo "  Basic Setup (~\$34/month):"
    echo "    - 1x cx21 control plane:  €5.18"
    echo "    - 2x cx31 workers:        €19.84"
    echo "    - 50GB volume:            €2.35"
    echo "    - 100GB volume:           €4.70"
    echo "    - 10GB volume:            €0.47"
    echo "    - Network:                ~€1.00"
    echo "    - Cloudflare:             Free"
    echo ""
    echo "  HA Setup (~\$55/month):"
    echo "    - 3x cx21 control plane:  €15.54"
    echo "    - 3x cx31 workers:        €29.76"
    echo "    - Volumes + Network:      ~€10"
    echo ""
}

show_next_steps() {
    echo ""
    log_info "=== Next Steps to Deploy Enclii ==="
    echo ""
    echo "1. Get API Tokens:"
    echo "   - Hetzner: https://console.hetzner.cloud/projects/<project>/security/tokens"
    echo "   - Cloudflare: https://dash.cloudflare.com/profile/api-tokens"
    echo "   - R2: https://dash.cloudflare.com/<account>/r2/api-tokens"
    echo ""
    echo "2. Update terraform.tfvars with real values"
    echo ""
    echo "3. Run terraform apply:"
    echo "   cd $TERRAFORM_DIR && terraform apply"
    echo ""
    echo "4. Deploy Kubernetes resources:"
    echo "   kubectl apply -k infra/k8s/production"
    echo ""
    echo "5. Create secrets:"
    echo "   - Database credentials"
    echo "   - Redis password"
    echo "   - R2 credentials"
    echo "   - JWT signing keys"
    echo ""
    echo "6. Deploy services:"
    echo "   - Switchyard API"
    echo "   - Switchyard UI"
    echo "   - Janua"
    echo ""
    echo "7. Test end-to-end authentication flow"
    echo ""
}

main() {
    echo "=========================================="
    echo "  Enclii Terraform Validation"
    echo "=========================================="
    echo ""

    check_prerequisites

    TFVARS_OK=true
    check_tfvars || TFVARS_OK=false

    terraform_init
    terraform_validate

    if [ "$TFVARS_OK" = true ]; then
        terraform_plan
    fi

    show_cost_estimate
    show_next_steps

    echo ""
    log_success "Validation complete!"
}

main "$@"
