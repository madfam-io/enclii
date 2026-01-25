#!/bin/bash
# validate-network-ports.sh
# Validates that NetworkPolicy ports match Deployment container ports
# for all Enclii-managed services.
#
# Usage:
#   ./scripts/validate-network-ports.sh           # Check and report mismatches
#   ./scripts/validate-network-ports.sh --fix     # Report only (no auto-fix, manual review required)
#
# Exit codes:
#   0 - All ports match
#   1 - Mismatches found
#   2 - Script error

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track mismatches
MISMATCHES=0
CHECKED=0

echo "🔍 NetworkPolicy Port Consistency Check"
echo "========================================"
echo ""

# Get all namespaces with Enclii-managed resources
# Look for namespaces with services that have enclii.dev/managed-by label
NAMESPACES=$(kubectl get deployments -A -l enclii.dev/managed-by=switchyard -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null | sort -u)

if [ -z "$NAMESPACES" ]; then
    echo "ℹ️  No Enclii-managed deployments found"
    echo "   Looking in common namespaces..."
    NAMESPACES="enclii janua"
fi

for ns in $NAMESPACES; do
    # Skip if namespace doesn't exist
    if ! kubectl get namespace "$ns" &>/dev/null; then
        continue
    fi

    echo "📁 Namespace: $ns"

    # Get all deployments in namespace
    DEPLOYMENTS=$(kubectl get deployments -n "$ns" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null)

    for deploy in $DEPLOYMENTS; do
        # Skip if empty
        [ -z "$deploy" ] && continue

        # Get container port from deployment
        CONTAINER_PORT=$(kubectl get deployment "$deploy" -n "$ns" -o jsonpath='{.spec.template.spec.containers[0].ports[0].containerPort}' 2>/dev/null || echo "")

        # Skip if no container port defined
        if [ -z "$CONTAINER_PORT" ]; then
            echo "  ⚠️  $deploy: No containerPort defined (skipped)"
            continue
        fi

        # Get NetworkPolicy port (ingress policy)
        NP_NAME="${deploy}-ingress"
        NP_PORT=$(kubectl get networkpolicy "$NP_NAME" -n "$ns" -o jsonpath='{.spec.ingress[0].ports[0].port}' 2>/dev/null || echo "")

        # Skip if no NetworkPolicy exists
        if [ -z "$NP_PORT" ]; then
            echo "  ℹ️  $deploy: No NetworkPolicy found (containerPort=$CONTAINER_PORT)"
            continue
        fi

        ((CHECKED++))

        # Compare ports
        if [ "$CONTAINER_PORT" != "$NP_PORT" ]; then
            echo -e "  ${RED}❌ $deploy: MISMATCH${NC}"
            echo "     Container port: $CONTAINER_PORT"
            echo "     NetworkPolicy port: $NP_PORT"
            echo "     Fix: kubectl patch networkpolicy $NP_NAME -n $ns --type='json' -p='[{\"op\": \"replace\", \"path\": \"/spec/ingress/0/ports/0/port\", \"value\": $CONTAINER_PORT}]'"
            ((MISMATCHES++))
        else
            echo -e "  ${GREEN}✅ $deploy: OK${NC} (port=$CONTAINER_PORT)"
        fi
    done
    echo ""
done

echo "========================================"
echo "Summary:"
echo "  Checked: $CHECKED deployments"
echo "  Mismatches: $MISMATCHES"
echo ""

if [ $MISMATCHES -gt 0 ]; then
    echo -e "${RED}❌ Port consistency check FAILED${NC}"
    echo ""
    echo "To fix mismatches, run the kubectl patch commands shown above."
    echo "Root cause: Service may be using PORT env var instead of ENCLII_PORT."
    echo "Permanent fix: Ensure services set ENCLII_PORT or PORT in their env vars."
    exit 1
else
    echo -e "${GREEN}✅ All NetworkPolicy ports match container ports${NC}"
    exit 0
fi
