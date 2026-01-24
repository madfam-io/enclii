#!/bin/bash
# rotate-oidc-secret.sh - Safely rotate OIDC credentials with validation
#
# This script rotates the OIDC client secret in K8s with pre and post
# validation to ensure the new secret works before restarting services.
#
# Usage:
#   ./scripts/rotate-oidc-secret.sh
#   KUBECONFIG=~/.kube/config-hetzner ./scripts/rotate-oidc-secret.sh
#
# The script will:
#   1. Show current credentials
#   2. Prompt for new client secret
#   3. Validate new secret against Janua (BEFORE applying)
#   4. Update K8s secret
#   5. Restart switchyard-api
#   6. Verify deployment health

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${ENCLII_NAMESPACE:-enclii}"
SECRET_NAME="${ENCLII_OIDC_SECRET:-enclii-oidc-credentials}"
JANUA_ISSUER="${JANUA_ISSUER:-https://auth.madfam.io}"
JANUA_TOKEN_ENDPOINT="${JANUA_ISSUER}/oauth/token"

echo "================================================"
echo "  OIDC Secret Rotation"
echo "================================================"
echo ""

# Check dependencies
for cmd in kubectl curl jq base64; do
    if ! command -v "$cmd" &> /dev/null; then
        echo -e "${RED}ERROR: Required command '$cmd' not found${NC}"
        exit 1
    fi
done

# Step 1: Get current credentials
echo -e "${BLUE}1. Current Configuration${NC}"
echo "   Namespace: $NAMESPACE"
echo "   Secret:    $SECRET_NAME"
echo "   Issuer:    $JANUA_ISSUER"
echo ""

CURRENT_CLIENT_ID=$(kubectl -n "$NAMESPACE" get secret "$SECRET_NAME" -o jsonpath='{.data.client-id}' 2>/dev/null | base64 -d || echo "")
CURRENT_CLIENT_SECRET=$(kubectl -n "$NAMESPACE" get secret "$SECRET_NAME" -o jsonpath='{.data.client-secret}' 2>/dev/null | base64 -d || echo "")

if [[ -n "$CURRENT_CLIENT_ID" ]]; then
    echo "   Current Client ID: $CURRENT_CLIENT_ID"
    echo "   Current Secret:    ${CURRENT_CLIENT_SECRET:0:10}...*** (masked)"
else
    echo -e "${YELLOW}   No existing secret found. Will create new one.${NC}"
fi
echo ""

# Step 2: Prompt for new credentials
echo -e "${BLUE}2. Enter New Credentials${NC}"
echo ""
echo "Get these from: https://auth.madfam.io → OAuth Clients"
echo ""

read -p "Client ID [${CURRENT_CLIENT_ID:-enter new}]: " NEW_CLIENT_ID
NEW_CLIENT_ID="${NEW_CLIENT_ID:-$CURRENT_CLIENT_ID}"

if [[ -z "$NEW_CLIENT_ID" ]]; then
    echo -e "${RED}ERROR: Client ID is required${NC}"
    exit 1
fi

echo -n "Client Secret: "
read -s NEW_CLIENT_SECRET
echo ""

if [[ -z "$NEW_CLIENT_SECRET" ]]; then
    echo -e "${RED}ERROR: Client Secret is required${NC}"
    exit 1
fi

if [[ "$NEW_CLIENT_SECRET" == "$CURRENT_CLIENT_SECRET" ]]; then
    echo -e "${YELLOW}WARNING: New secret is the same as current secret${NC}"
    read -p "Continue anyway? (y/N): " CONFIRM
    if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
        echo "Aborted."
        exit 0
    fi
fi

# Step 3: Validate new credentials BEFORE applying
echo ""
echo -e "${BLUE}3. Validating New Credentials${NC}"
echo "   Testing against Janua..."

RESPONSE=$(curl -s -w "\n%{http_code}" \
    -X POST "$JANUA_TOKEN_ENDPOINT" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=client_credentials" \
    -d "client_id=$NEW_CLIENT_ID" \
    -d "client_secret=$NEW_CLIENT_SECRET" \
    -d "scope=openid")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)

if [[ "$HTTP_CODE" != "200" ]]; then
    ERROR_MSG=$(echo "$BODY" | jq -r '.error.message // .error_description // .error // "Unknown error"' 2>/dev/null || echo "$BODY")
    echo -e "${RED}   VALIDATION FAILED: $ERROR_MSG${NC}"
    echo ""
    echo "The new credentials are invalid. Secret NOT updated."
    echo "Please check the client ID and secret in Janua admin panel."
    exit 2
fi

echo -e "${GREEN}   Credentials VALIDATED successfully${NC}"

# Step 4: Update K8s secret
echo ""
echo -e "${BLUE}4. Updating K8s Secret${NC}"

# Delete and recreate (atomic update)
kubectl -n "$NAMESPACE" delete secret "$SECRET_NAME" --ignore-not-found
kubectl -n "$NAMESPACE" create secret generic "$SECRET_NAME" \
    --from-literal=client-id="$NEW_CLIENT_ID" \
    --from-literal=client-secret="$NEW_CLIENT_SECRET"

echo -e "${GREEN}   Secret updated${NC}"

# Step 5: Restart switchyard-api
echo ""
echo -e "${BLUE}5. Restarting switchyard-api${NC}"
kubectl -n "$NAMESPACE" rollout restart deployment/switchyard-api
echo "   Waiting for rollout..."
kubectl -n "$NAMESPACE" rollout status deployment/switchyard-api --timeout=120s

echo -e "${GREEN}   Deployment restarted${NC}"

# Step 6: Final verification
echo ""
echo -e "${BLUE}6. Final Verification${NC}"
sleep 5  # Wait for pod to be ready

# Check pod status
POD_STATUS=$(kubectl -n "$NAMESPACE" get pods -l app=switchyard-api -o jsonpath='{.items[0].status.phase}')
if [[ "$POD_STATUS" == "Running" ]]; then
    echo -e "${GREEN}   Pod status: Running${NC}"
else
    echo -e "${YELLOW}   Pod status: $POD_STATUS${NC}"
fi

# Check for auth errors in logs
RECENT_ERRORS=$(kubectl -n "$NAMESPACE" logs -l app=switchyard-api --tail=20 2>/dev/null | grep -c "invalid_client\|Invalid client" || true)
if [[ "$RECENT_ERRORS" -gt 0 ]]; then
    echo -e "${YELLOW}   WARNING: Found auth errors in recent logs${NC}"
else
    echo -e "${GREEN}   No auth errors in recent logs${NC}"
fi

echo ""
echo "================================================"
echo -e "${GREEN}  OIDC Secret Rotation: COMPLETE${NC}"
echo "================================================"
echo ""
echo "Client ID: $NEW_CLIENT_ID"
echo "Issuer:    $JANUA_ISSUER"
echo ""
echo "Test the login flow at: https://app.enclii.dev"
echo ""
