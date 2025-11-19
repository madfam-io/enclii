#!/bin/bash
set -e

echo "==================================================================="
echo "Enclii Build Pipeline - Tool Installation Script"
echo "==================================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo -e "${YELLOW}Warning: Running as root. Some installations may need adjustment.${NC}"
fi

# Detect OS
OS="unknown"
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
fi

echo "Detected OS: $OS"
echo ""

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 1. Install Docker
echo "==================================================================="
echo "1. Checking Docker"
echo "==================================================================="
if command_exists docker; then
    echo -e "${GREEN}✓ Docker is already installed${NC}"
    docker --version

    # Check if Docker daemon is running
    if docker info >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Docker daemon is running${NC}"
    else
        echo -e "${YELLOW}⚠ Docker is installed but daemon is not running${NC}"
        echo "  Start Docker with: sudo systemctl start docker"
    fi
else
    echo -e "${YELLOW}⚠ Docker not found. Installing...${NC}"

    if [ "$OS" == "linux" ]; then
        # Install Docker on Linux
        curl -fsSL https://get.docker.com -o get-docker.sh
        sh get-docker.sh
        sudo usermod -aG docker $USER
        echo -e "${GREEN}✓ Docker installed. Please log out and back in for group changes to take effect.${NC}"
        rm get-docker.sh
    elif [ "$OS" == "macos" ]; then
        echo -e "${YELLOW}Please install Docker Desktop from: https://www.docker.com/products/docker-desktop${NC}"
        echo "After installation, run this script again."
        exit 1
    fi
fi
echo ""

# 2. Install Pack CLI
echo "==================================================================="
echo "2. Checking Pack CLI (Cloud Native Buildpacks)"
echo "==================================================================="
if command_exists pack; then
    echo -e "${GREEN}✓ Pack CLI is already installed${NC}"
    pack --version
else
    echo -e "${YELLOW}⚠ Pack CLI not found. Installing...${NC}"

    PACK_VERSION="v0.32.1"

    if [ "$OS" == "linux" ]; then
        echo "Installing Pack CLI for Linux..."
        curl -sSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-linux.tgz" | \
            sudo tar -C /usr/local/bin/ --no-same-owner -xzv pack
        echo -e "${GREEN}✓ Pack CLI installed${NC}"
    elif [ "$OS" == "macos" ]; then
        if command_exists brew; then
            echo "Installing Pack CLI via Homebrew..."
            brew install buildpacks/tap/pack
            echo -e "${GREEN}✓ Pack CLI installed${NC}"
        else
            echo "Installing Pack CLI manually..."
            curl -sSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-macos.tgz" | \
                sudo tar -C /usr/local/bin/ --no-same-owner -xzv pack
            echo -e "${GREEN}✓ Pack CLI installed${NC}"
        fi
    fi

    pack --version
fi
echo ""

# 3. Create build directories
echo "==================================================================="
echo "3. Creating Build Directories"
echo "==================================================================="
BUILD_WORK_DIR="${BUILD_WORK_DIR:-/tmp/enclii-builds}"
BUILD_CACHE_DIR="${BUILD_CACHE_DIR:-/var/cache/enclii-builds}"

echo "Creating build work directory: $BUILD_WORK_DIR"
mkdir -p "$BUILD_WORK_DIR"
chmod 755 "$BUILD_WORK_DIR"

echo "Creating build cache directory: $BUILD_CACHE_DIR"
sudo mkdir -p "$BUILD_CACHE_DIR" 2>/dev/null || mkdir -p "$BUILD_CACHE_DIR"
sudo chmod 777 "$BUILD_CACHE_DIR" 2>/dev/null || chmod 777 "$BUILD_CACHE_DIR"

echo -e "${GREEN}✓ Build directories created${NC}"
echo ""

# 4. Configure Docker Registry
echo "==================================================================="
echo "4. Docker Registry Configuration"
echo "==================================================================="
echo "To push images, you need to authenticate with your container registry."
echo ""
echo "For GitHub Container Registry (ghcr.io):"
echo "  docker login ghcr.io -u YOUR_GITHUB_USERNAME -p YOUR_GITHUB_TOKEN"
echo ""
echo "For Docker Hub:"
echo "  docker login -u YOUR_DOCKER_USERNAME"
echo ""
echo "For other registries:"
echo "  docker login YOUR_REGISTRY_URL -u USERNAME -p PASSWORD"
echo ""

if [ -f "$HOME/.docker/config.json" ]; then
    echo -e "${GREEN}✓ Docker config found at $HOME/.docker/config.json${NC}"
else
    echo -e "${YELLOW}⚠ No Docker config found. Run 'docker login' after this script.${NC}"
fi
echo ""

# 5. Verify installation
echo "==================================================================="
echo "5. Verification"
echo "==================================================================="

ERRORS=0

# Check Docker
if command_exists docker && docker info >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Docker: OK${NC}"
else
    echo -e "${RED}✗ Docker: FAIL${NC}"
    ERRORS=$((ERRORS + 1))
fi

# Check Pack
if command_exists pack; then
    echo -e "${GREEN}✓ Pack CLI: OK${NC}"
else
    echo -e "${RED}✗ Pack CLI: FAIL${NC}"
    ERRORS=$((ERRORS + 1))
fi

# Check build directories
if [ -d "$BUILD_WORK_DIR" ] && [ -w "$BUILD_WORK_DIR" ]; then
    echo -e "${GREEN}✓ Build work directory: OK${NC}"
else
    echo -e "${RED}✗ Build work directory: FAIL${NC}"
    ERRORS=$((ERRORS + 1))
fi

if [ -d "$BUILD_CACHE_DIR" ] && [ -w "$BUILD_CACHE_DIR" ]; then
    echo -e "${GREEN}✓ Build cache directory: OK${NC}"
else
    echo -e "${RED}✗ Build cache directory: FAIL${NC}"
    ERRORS=$((ERRORS + 1))
fi

echo ""
if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}==================================================================="
    echo "✓ All build tools installed successfully!"
    echo "===================================================================${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Configure registry authentication: docker login ghcr.io"
    echo "2. Set environment variables in .env"
    echo "3. Start the Switchyard API: make run-switchyard"
    echo "4. Test a build: curl -X POST http://localhost:8080/v1/services/{id}/build"
else
    echo -e "${RED}==================================================================="
    echo "✗ $ERRORS error(s) occurred during installation"
    echo "===================================================================${NC}"
    echo "Please fix the errors above and run this script again."
    exit 1
fi
