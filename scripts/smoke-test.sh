#!/bin/bash
# Post-Deployment Smoke Tests
# Run immediately after deployment: ./scripts/smoke-test.sh

set -euo pipefail

TIMEOUT=60
INTERVAL=5

# OIDC issuer URL - defaults to api.janua.dev (The Product)
# Override with ENCLII_OIDC_ISSUER for custom deployments (e.g., auth.madfam.io)
OIDC_ISSUER_URL="${ENCLII_OIDC_ISSUER:-https://api.janua.dev}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo "=========================================="
echo "Post-Deployment Smoke Tests"
echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "=========================================="

wait_for_ready() {
    local url="$1"
    local name="$2"
    local elapsed=0

    echo -n "Waiting for $name... "

    while [[ $elapsed -lt $TIMEOUT ]]; do
        local code
        code=$(curl -sL -o /dev/null -w "%{http_code}" --max-time 5 "$url" 2>/dev/null || echo "000")

        if [[ "$code" =~ ^2[0-9][0-9]$ ]] || [[ "$code" == "302" ]]; then
            echo -e "${GREEN}OK${NC} (${elapsed}s, HTTP $code)"
            return 0
        fi
        sleep $INTERVAL
        elapsed=$((elapsed + INTERVAL))
    done

    echo -e "${RED}TIMEOUT${NC} after ${TIMEOUT}s"
    return 1
}

# Core smoke tests
FAILED=0

echo ""
echo "--- Core Services ---"
wait_for_ready "https://api.enclii.dev/health" "API" || FAILED=$((FAILED+1))
wait_for_ready "https://app.enclii.dev" "Dashboard" || FAILED=$((FAILED+1))
wait_for_ready "https://enclii.dev" "Landing Page" || FAILED=$((FAILED+1))

echo ""
echo "--- Authentication ---"
echo -n "Testing API login redirect... "
LOGIN_RESPONSE=$(curl -sI "https://api.enclii.dev/v1/auth/login" 2>/dev/null | grep -i "location:" || echo "")
if [[ "$LOGIN_RESPONSE" == *"janua"* ]] || [[ "$LOGIN_RESPONSE" == *"${OIDC_ISSUER_URL}"* ]]; then
    echo -e "${GREEN}OK${NC} (redirects to OIDC)"
else
    echo -e "${RED}FAIL${NC}: No OIDC redirect found"
    FAILED=$((FAILED+1))
fi

echo -n "Testing OIDC discovery (${OIDC_ISSUER_URL})... "
OIDC_ISSUER=$(curl -s "${OIDC_ISSUER_URL}/.well-known/openid-configuration" 2>/dev/null | grep -o '"issuer":"[^"]*"' || echo "")
if [[ -n "$OIDC_ISSUER" ]]; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAIL${NC}: OIDC discovery failed"
    FAILED=$((FAILED+1))
fi

echo ""
echo "=========================================="
if [[ $FAILED -gt 0 ]]; then
    echo -e "${RED}SMOKE TESTS FAILED: $FAILED test(s)${NC}"
    echo "Consider rollback!"
    echo "=========================================="
    exit 1
fi

echo -e "${GREEN}All smoke tests passed${NC}"
echo "=========================================="
exit 0
