#!/bin/bash
# Production Health Check Script
# Run: ./scripts/health-check.sh
# Cron: */5 * * * * /path/to/health-check.sh

set -euo pipefail

# Configuration
SLACK_WEBHOOK="${ENCLII_SLACK_WEBHOOK:-}"
ENDPOINTS=(
    "https://enclii.dev|Landing Page"
    "https://app.enclii.dev|Dashboard"
    "https://api.enclii.dev/health|API Health"
    "https://docs.enclii.dev|Documentation"
    "https://auth.madfam.io/.well-known/openid-configuration|OIDC Discovery"
)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

# Results
FAILED=()
PASSED=()

check_endpoint() {
    local url="${1%%|*}"
    local name="${1##*|}"

    local http_code
    http_code=$(curl -sL -o /dev/null -w "%{http_code}" --max-time 10 "$url" 2>/dev/null || echo "000")

    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]] || [[ "$http_code" == "302" ]]; then
        PASSED+=("$name: $http_code")
        echo -e "${GREEN}✓${NC} $name ($url): $http_code"
        return 0
    else
        FAILED+=("$name: $http_code ($url)")
        echo -e "${RED}✗${NC} $name ($url): $http_code"
        return 1
    fi
}

send_alert() {
    local message="$1"

    if [[ -n "$SLACK_WEBHOOK" ]]; then
        curl -s -X POST "$SLACK_WEBHOOK" \
            -H 'Content-type: application/json' \
            -d "{\"text\": \"$message\"}" > /dev/null
    fi

    echo -e "${RED}ALERT:${NC} $message"
}

main() {
    echo "=========================================="
    echo "Enclii Production Health Check"
    echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    echo "=========================================="

    for endpoint in "${ENDPOINTS[@]}"; do
        check_endpoint "$endpoint" || true
    done

    echo ""
    echo "=========================================="
    echo "Summary: ${#PASSED[@]} passed, ${#FAILED[@]} failed"
    echo "=========================================="

    if [[ ${#FAILED[@]} -gt 0 ]]; then
        local alert_msg="PRODUCTION ALERT: ${#FAILED[@]} service(s) failing\n"
        for failure in "${FAILED[@]}"; do
            alert_msg+="- $failure\n"
        done
        send_alert "$alert_msg"
        exit 1
    fi

    echo -e "${GREEN}All services healthy${NC}"
    exit 0
}

main "$@"
