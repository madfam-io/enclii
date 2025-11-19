#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "==================================================================="
echo "Enclii Build Pipeline - Test Script"
echo "==================================================================="
echo ""

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
TEST_REPO="${TEST_REPO:-https://github.com/heroku/node-js-sample.git}"
TEST_SHA="${TEST_SHA:-main}"

# Step 1: Check API is running
echo "1. Checking if API is running..."
if curl -sf "$API_URL/health" > /dev/null; then
    echo -e "${GREEN}✓ API is running${NC}"
else
    echo -e "${RED}✗ API is not running at $API_URL${NC}"
    echo "Start the API with: make run-switchyard"
    exit 1
fi

# Step 2: Check build tools status
echo ""
echo "2. Checking build tools status..."
TOOLS_RESPONSE=$(curl -s "$API_URL/v1/build/status")
TOOLS_STATUS=$(echo "$TOOLS_RESPONSE" | jq -r '.status' 2>/dev/null || echo "unknown")

if [ "$TOOLS_STATUS" == "healthy" ]; then
    echo -e "${GREEN}✓ Build tools are available${NC}"
    echo "$TOOLS_RESPONSE" | jq '.build_pipeline'
else
    echo -e "${YELLOW}⚠ Build tools are not fully available${NC}"
    echo "$TOOLS_RESPONSE" | jq '.'
    echo ""
    echo "To install build tools, run:"
    echo "  ./scripts/setup-build-tools.sh"
    echo ""
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Step 3: Register test user
echo ""
echo "3. Registering test user..."
REGISTER_RESPONSE=$(curl -s -X POST "$API_URL/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test-'$(date +%s)'@example.com",
        "password": "TestPassword123!",
        "name": "Test User"
    }')

ACCESS_TOKEN=$(echo "$REGISTER_RESPONSE" | jq -r '.access_token' 2>/dev/null)

if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
    echo -e "${GREEN}✓ User registered successfully${NC}"
else
    echo -e "${RED}✗ Failed to register user${NC}"
    echo "$REGISTER_RESPONSE" | jq '.'
    exit 1
fi

# Step 4: Create test project
echo ""
echo "4. Creating test project..."
PROJECT_SLUG="test-project-$(date +%s)"
CREATE_PROJECT_RESPONSE=$(curl -s -X POST "$API_URL/v1/projects" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "Test Project",
        "slug": "'$PROJECT_SLUG'"
    }')

PROJECT_ID=$(echo "$CREATE_PROJECT_RESPONSE" | jq -r '.id' 2>/dev/null)

if [ -n "$PROJECT_ID" ] && [ "$PROJECT_ID" != "null" ]; then
    echo -e "${GREEN}✓ Project created: $PROJECT_SLUG${NC}"
else
    echo -e "${RED}✗ Failed to create project${NC}"
    echo "$CREATE_PROJECT_RESPONSE" | jq '.'
    exit 1
fi

# Step 5: Create test service
echo ""
echo "5. Creating test service..."
CREATE_SERVICE_RESPONSE=$(curl -s -X POST "$API_URL/v1/projects/$PROJECT_SLUG/services" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "test-app",
        "git_repo": "'$TEST_REPO'",
        "build_config": {
            "type": "auto"
        }
    }')

SERVICE_ID=$(echo "$CREATE_SERVICE_RESPONSE" | jq -r '.id' 2>/dev/null)

if [ -n "$SERVICE_ID" ] && [ "$SERVICE_ID" != "null" ]; then
    echo -e "${GREEN}✓ Service created: $SERVICE_ID${NC}"
else
    echo -e "${RED}✗ Failed to create service${NC}"
    echo "$CREATE_SERVICE_RESPONSE" | jq '.'
    exit 1
fi

# Step 6: Trigger build
echo ""
echo "6. Triggering build..."
echo "   Repository: $TEST_REPO"
echo "   Git SHA: $TEST_SHA"
BUILD_RESPONSE=$(curl -s -X POST "$API_URL/v1/services/$SERVICE_ID/build" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "git_sha": "'$TEST_SHA'"
    }')

RELEASE_ID=$(echo "$BUILD_RESPONSE" | jq -r '.id' 2>/dev/null)

if [ -n "$RELEASE_ID" ] && [ "$RELEASE_ID" != "null" ]; then
    echo -e "${GREEN}✓ Build triggered: $RELEASE_ID${NC}"
    echo "   Status: $(echo "$BUILD_RESPONSE" | jq -r '.status')"
    echo "   Image: $(echo "$BUILD_RESPONSE" | jq -r '.image_uri')"
else
    echo -e "${RED}✗ Failed to trigger build${NC}"
    echo "$BUILD_RESPONSE" | jq '.'
    exit 1
fi

# Step 7: Monitor build progress
echo ""
echo "7. Monitoring build progress..."
echo "   This may take 3-8 minutes for first build..."
echo ""

MAX_WAIT=600  # 10 minutes
WAIT_INTERVAL=5
ELAPSED=0

while [ $ELAPSED -lt $MAX_WAIT ]; do
    RELEASES_RESPONSE=$(curl -s "$API_URL/v1/services/$SERVICE_ID/releases" \
        -H "Authorization: Bearer $ACCESS_TOKEN")

    BUILD_STATUS=$(echo "$RELEASES_RESPONSE" | jq -r '.releases[0].status' 2>/dev/null)

    if [ "$BUILD_STATUS" == "ready" ]; then
        echo -e "${GREEN}✓ Build completed successfully!${NC}"
        echo ""
        echo "$RELEASES_RESPONSE" | jq '.releases[0]'
        break
    elif [ "$BUILD_STATUS" == "failed" ]; then
        echo -e "${RED}✗ Build failed${NC}"
        echo ""
        echo "$RELEASES_RESPONSE" | jq '.releases[0]'
        exit 1
    else
        echo -n "."
        sleep $WAIT_INTERVAL
        ELAPSED=$((ELAPSED + WAIT_INTERVAL))
    fi
done

if [ $ELAPSED -ge $MAX_WAIT ]; then
    echo -e "${YELLOW}⚠ Build timeout after $MAX_WAIT seconds${NC}"
    echo "Check API logs for details: tail -f logs/switchyard-api.log"
    exit 1
fi

# Step 8: Summary
echo ""
echo "==================================================================="
echo -e "${GREEN}✓ Build pipeline test completed successfully!${NC}"
echo "==================================================================="
echo ""
echo "Results:"
echo "  Project: $PROJECT_SLUG"
echo "  Service: $SERVICE_ID"
echo "  Release: $RELEASE_ID"
echo "  Status: $(echo "$RELEASES_RESPONSE" | jq -r '.releases[0].status')"
echo "  Image: $(echo "$RELEASES_RESPONSE" | jq -r '.releases[0].image_uri')"
echo ""
echo "Next steps:"
echo "  1. Deploy the service: curl -X POST $API_URL/v1/services/$SERVICE_ID/deploy"
echo "  2. View releases: curl $API_URL/v1/services/$SERVICE_ID/releases"
echo "  3. Check logs: tail -f logs/switchyard-api.log"
echo ""
