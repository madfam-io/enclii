#!/usr/bin/env bash
# Integration Test Runner for Enclii
# This script runs integration tests against a Kubernetes cluster

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
TEST_NAMESPACE="${TEST_NAMESPACE:-enclii-integration-tests}"
TEST_TIMEOUT="${TEST_TIMEOUT:-30m}"
CLEANUP="${CLEANUP:-true}"

# Test suites
SUITE_PVC="${SUITE_PVC:-true}"
SUITE_VOLUMES="${SUITE_VOLUMES:-true}"
SUITE_DOMAINS="${SUITE_DOMAINS:-true}"
SUITE_ROUTES="${SUITE_ROUTES:-true}"

function print_header() {
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${GREEN}========================================${NC}"
}

function print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

function print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

function print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

function check_prerequisites() {
    print_header "Checking Prerequisites"

    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl not found. Please install kubectl."
        exit 1
    fi
    print_success "kubectl found: $(kubectl version --client --short 2>/dev/null | head -1)"

    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster. Check your KUBECONFIG."
        exit 1
    fi
    print_success "Connected to cluster: $(kubectl config current-context)"

    # Check Go
    if ! command -v go &> /dev/null; then
        print_error "Go not found. Please install Go."
        exit 1
    fi
    print_success "Go found: $(go version)"

    # Check if cert-manager is installed
    if kubectl get clusterissuer letsencrypt-staging &> /dev/null; then
        print_success "cert-manager detected (letsencrypt-staging issuer found)"
    else
        print_info "cert-manager not detected. Some TLS tests may fail."
    fi

    # Check if nginx ingress controller is installed
    if kubectl get deployment -n ingress-nginx ingress-nginx-controller &> /dev/null; then
        print_success "nginx-ingress-controller detected"
    else
        print_info "nginx-ingress-controller not detected. Ingress tests may fail."
    fi

    echo ""
}

function cleanup_namespaces() {
    print_header "Cleaning Up Test Namespaces"

    # List all test namespaces
    TEST_NAMESPACES=$(kubectl get namespaces -o jsonpath='{.items[?(@.metadata.labels.test=="integration")].metadata.name}' 2>/dev/null || echo "")

    if [ -z "$TEST_NAMESPACES" ]; then
        print_info "No test namespaces to clean up"
        return
    fi

    for ns in $TEST_NAMESPACES; do
        print_info "Deleting namespace: $ns"
        kubectl delete namespace "$ns" --timeout=60s &
    done

    # Wait for all deletions
    wait

    print_success "Cleanup complete"
    echo ""
}

function run_test_suite() {
    local suite_name=$1
    local test_file=$2

    print_header "Running Test Suite: $suite_name"

    if ! go test -v \
        -timeout "$TEST_TIMEOUT" \
        -run "^Test" \
        "./$test_file" 2>&1 | tee "/tmp/enclii-test-$suite_name.log"; then
        print_error "Test suite failed: $suite_name"
        return 1
    fi

    print_success "Test suite passed: $suite_name"
    echo ""
    return 0
}

function run_all_tests() {
    print_header "Running All Integration Tests"

    local failed_suites=()

    # Run PVC persistence tests
    if [ "$SUITE_PVC" = "true" ]; then
        if ! run_test_suite "pvc-persistence" "pvc_persistence_test.go"; then
            failed_suites+=("pvc-persistence")
        fi
    fi

    # Run service volume tests
    if [ "$SUITE_VOLUMES" = "true" ]; then
        if ! run_test_suite "service-volumes" "service_volumes_test.go"; then
            failed_suites+=("service-volumes")
        fi
    fi

    # Run custom domain tests
    if [ "$SUITE_DOMAINS" = "true" ]; then
        if ! run_test_suite "custom-domains" "custom_domain_test.go"; then
            failed_suites+=("custom-domains")
        fi
    fi

    # Run route tests
    if [ "$SUITE_ROUTES" = "true" ]; then
        if ! run_test_suite "routes" "routes_test.go"; then
            failed_suites+=("routes")
        fi
    fi

    # Summary
    echo ""
    print_header "Test Summary"

    if [ ${#failed_suites[@]} -eq 0 ]; then
        print_success "All test suites passed! ✅"
        return 0
    else
        print_error "Failed test suites:"
        for suite in "${failed_suites[@]}"; do
            echo "  - $suite"
        done
        return 1
    fi
}

function show_usage() {
    cat << EOF
Enclii Integration Test Runner

Usage: $0 [OPTIONS]

Options:
  -h, --help              Show this help message
  -c, --cleanup           Clean up test namespaces before running (default: true)
  --no-cleanup            Don't clean up test namespaces
  --suite-pvc             Run PVC persistence tests (default: true)
  --suite-volumes         Run service volume tests (default: true)
  --suite-domains         Run custom domain tests (default: true)
  --suite-routes          Run route tests (default: true)
  --timeout DURATION      Test timeout duration (default: 30m)

Environment Variables:
  KUBECONFIG              Path to kubeconfig file
  TEST_NAMESPACE          Namespace for tests (default: enclii-integration-tests)
  TEST_TIMEOUT            Test timeout (default: 30m)
  CLEANUP                 Clean up before running (true/false)

Examples:
  # Run all tests
  $0

  # Run only custom domain tests
  $0 --suite-domains --no-cleanup

  # Run with custom timeout
  TEST_TIMEOUT=1h $0

  # Run specific suite
  SUITE_PVC=false SUITE_VOLUMES=false SUITE_ROUTES=false $0
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -c|--cleanup)
            CLEANUP=true
            shift
            ;;
        --no-cleanup)
            CLEANUP=false
            shift
            ;;
        --suite-pvc)
            SUITE_PVC=true
            shift
            ;;
        --suite-volumes)
            SUITE_VOLUMES=true
            shift
            ;;
        --suite-domains)
            SUITE_DOMAINS=true
            shift
            ;;
        --suite-routes)
            SUITE_ROUTES=true
            shift
            ;;
        --timeout)
            TEST_TIMEOUT="$2"
            shift 2
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    print_header "Enclii Integration Test Runner"
    echo "KUBECONFIG: $KUBECONFIG"
    echo "TEST_TIMEOUT: $TEST_TIMEOUT"
    echo "CLEANUP: $CLEANUP"
    echo ""

    # Check prerequisites
    check_prerequisites

    # Cleanup if requested
    if [ "$CLEANUP" = "true" ]; then
        cleanup_namespaces
    fi

    # Change to integration test directory
    cd "$(dirname "$0")"

    # Run tests
    if run_all_tests; then
        print_success "✅ All integration tests passed!"
        exit 0
    else
        print_error "❌ Some integration tests failed"
        exit 1
    fi
}

# Run main function
main
