#!/bin/bash
# =============================================================================
# Shared Logging Library for Enclii Scripts
# =============================================================================
#
# Usage: Source this file at the beginning of your script:
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "$SCRIPT_DIR/lib/logging.sh"
#
# Or if in a subdirectory:
#   source "$(dirname "$0")/../lib/logging.sh"
#
# Provides:
#   - Color variables: RED, GREEN, YELLOW, BLUE, PURPLE, CYAN, NC
#   - Logging functions: log_info, log_success, log_warn, log_error, log_debug
#   - Utility functions: confirm, banner, hr
# =============================================================================

# Prevent multiple inclusions
if [[ -n "${_ENCLII_LOGGING_LOADED:-}" ]]; then
    return 0
fi
_ENCLII_LOGGING_LOADED=1

# =============================================================================
# Color Definitions
# =============================================================================
# Use export so colors are available in subshells

export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export PURPLE='\033[0;35m'
export CYAN='\033[0;36m'
export BOLD='\033[1m'
export DIM='\033[2m'
export NC='\033[0m' # No Color / Reset

# Disable colors if not in a terminal or if NO_COLOR is set
if [[ ! -t 1 ]] || [[ -n "${NO_COLOR:-}" ]]; then
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    PURPLE=''
    CYAN=''
    BOLD=''
    DIM=''
    NC=''
fi

# =============================================================================
# Logging Functions
# =============================================================================

# Log an informational message (blue prefix)
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

# Log a success message (green checkmark)
log_success() {
    echo -e "${GREEN}[✓]${NC} $*"
}

# Log a warning message (yellow prefix)
log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

# Log an error message (red prefix, to stderr)
log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

# Log a debug message (only if DEBUG=1 or VERBOSE=1)
log_debug() {
    if [[ "${DEBUG:-0}" == "1" ]] || [[ "${VERBOSE:-0}" == "1" ]]; then
        echo -e "${DIM}[DEBUG]${NC} $*"
    fi
}

# Log a step in a multi-step process (numbered)
log_step() {
    local step_num="${1:-}"
    local total="${2:-}"
    local message="${3:-}"

    if [[ -n "$total" && -n "$message" ]]; then
        echo -e "${CYAN}[$step_num/$total]${NC} $message"
    else
        # If only one arg, treat as message without numbers
        echo -e "${CYAN}[*]${NC} $step_num"
    fi
}

# =============================================================================
# Utility Functions
# =============================================================================

# Prompt for confirmation (returns 0 for yes, 1 for no)
# Usage: confirm "Are you sure?" && do_something
confirm() {
    local prompt="${1:-Are you sure?}"
    local response

    echo -en "${YELLOW}$prompt${NC} [y/N]: "
    read -r response

    case "$response" in
        [yY][eE][sS]|[yY])
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Print a horizontal rule
hr() {
    local char="${1:--}"
    local width="${2:-60}"
    printf '%*s\n' "$width" '' | tr ' ' "$char"
}

# Print a section header
section() {
    local title="$1"
    echo ""
    hr "="
    echo -e "${BOLD}$title${NC}"
    hr "="
}

# Die with an error message
die() {
    log_error "$@"
    exit 1
}

# Check if a command exists
require_cmd() {
    local cmd="$1"
    local install_hint="${2:-}"

    if ! command -v "$cmd" &>/dev/null; then
        log_error "Required command not found: $cmd"
        if [[ -n "$install_hint" ]]; then
            echo "  Install with: $install_hint"
        fi
        exit 1
    fi
}

# Check if running as root (and optionally require it)
require_root() {
    if [[ $EUID -ne 0 ]]; then
        die "This script must be run as root (use sudo)"
    fi
}

# =============================================================================
# Banner / Branding
# =============================================================================

# Print the Enclii banner
enclii_banner() {
    local title="${1:-}"

    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                                                               ║${NC}"
    echo -e "${GREEN}║   ███████╗███╗   ██╗ ██████╗██╗     ██╗██╗                   ║${NC}"
    echo -e "${GREEN}║   ██╔════╝████╗  ██║██╔════╝██║     ██║██║                   ║${NC}"
    echo -e "${GREEN}║   █████╗  ██╔██╗ ██║██║     ██║     ██║██║                   ║${NC}"
    echo -e "${GREEN}║   ██╔══╝  ██║╚██╗██║██║     ██║     ██║██║                   ║${NC}"
    echo -e "${GREEN}║   ███████╗██║ ╚████║╚██████╗███████╗██║██║                   ║${NC}"
    echo -e "${GREEN}║   ╚══════╝╚═╝  ╚═══╝ ╚═════╝╚══════╝╚═╝╚═╝                   ║${NC}"
    echo -e "${GREEN}║                                                               ║${NC}"
    if [[ -n "$title" ]]; then
        printf "${GREEN}║${NC}           %-43s ${GREEN}║${NC}\n" "$title"
    fi
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# =============================================================================
# Progress Indicators
# =============================================================================

# Show a spinner while a command runs
# Usage: with_spinner "Installing dependencies..." npm install
with_spinner() {
    local message="$1"
    shift

    local spinstr='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local i=0

    # Start the command in background
    "$@" &
    local pid=$!

    # Show spinner while command runs
    echo -n "$message "
    while kill -0 "$pid" 2>/dev/null; do
        local temp=${spinstr:i++%${#spinstr}:1}
        echo -ne "\r$message ${CYAN}$temp${NC} "
        sleep 0.1
    done

    # Check exit status
    wait "$pid"
    local status=$?

    if [[ $status -eq 0 ]]; then
        echo -e "\r$message ${GREEN}✓${NC}  "
    else
        echo -e "\r$message ${RED}✗${NC}  "
    fi

    return $status
}
