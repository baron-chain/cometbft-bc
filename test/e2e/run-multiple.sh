#!/usr/bin/env bash

# Testnet BC Runner Script
# Runs multiple testnet manifests sequentially and handles failures gracefully.
# Usage: ./run_testnets.sh [OPTIONS] MANIFEST...

set -euo pipefail

# Configuration
readonly RUNNER="./build/runner"
readonly LOG_DIR="${LOG_DIR:-logs}"
readonly TIMEOUT="${TIMEOUT:-3600}"  # Default timeout 1 hour

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Helper functions
log() {
    echo -e "${2:-$NC}==>${NC} $1"
}

log_error() {
    log "$1" "$RED" >&2
}

log_success() {
    log "$1" "$GREEN"
}

log_warning() {
    log "$1" "$YELLOW"
}

show_usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS] MANIFEST...

Runs multiple testnet manifests sequentially and handles failures.

Options:
    -h, --help          Show this help message
    -t, --timeout NUM   Set timeout in seconds (default: 3600)
    -l, --log-dir DIR   Set log directory (default: ./logs)
    -v, --verbose       Enable verbose output
    -k, --keep-logs     Don't delete logs on success

Arguments:
    MANIFEST    One or more testnet manifest files to run

Environment variables:
    LOG_DIR     Override default log directory
    TIMEOUT     Override default timeout in seconds
EOF
}

# Process command line arguments
VERBOSE=0
KEEP_LOGS=0
POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -l|--log-dir)
            LOG_DIR="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=1
            shift
            ;;
        -k|--keep-logs)
            KEEP_LOGS=1
            shift
            ;;
        -*|--*)
            log_error "Unknown option $1"
            show_usage
            exit 1
            ;;
        *)
            POSITIONAL_ARGS+=("$1")
            shift
            ;;
    esac
done

set -- "${POSITIONAL_ARGS[@]}"

# Validate inputs
if [[ $# -eq 0 ]]; then
    log_error "No manifest files provided"
    show_usage
    exit 1
fi

# Ensure runner exists
if [[ ! -x $RUNNER ]]; then
    log_error "Runner not found or not executable: $RUNNER"
    exit 1
fi

# Create log directory
mkdir -p "$LOG_DIR"

# Initialize arrays for tracking
FAILED=()
SUCCESSFUL=()
TOTAL_START=$SECONDS

# Run testnets
for MANIFEST in "$@"; do
    if [[ ! -f $MANIFEST ]]; then
        log_error "Manifest file not found: $MANIFEST"
        continue
    fi

    START=$SECONDS
    MANIFEST_NAME=$(basename "$MANIFEST")
    LOG_FILE="${LOG_DIR}/${MANIFEST_NAME}.log"

    log "Running testnet $MANIFEST..."
    
    # Run with timeout
    if timeout "$TIMEOUT" "$RUNNER" -f "$MANIFEST" > "$LOG_FILE" 2>&1; then
        DURATION=$((SECONDS - START))
        log_success "Completed testnet $MANIFEST in ${DURATION}s"
        SUCCESSFUL+=("$MANIFEST")
        
        # Clean up logs if not keeping them
        if [[ $KEEP_LOGS -eq 0 ]]; then
            rm -f "$LOG_FILE"
        fi
    else
        EXIT_CODE=$?
        DURATION=$((SECONDS - START))
        
        # Handle timeout vs other failures
        if [[ $EXIT_CODE -eq 124 ]]; then
            log_error "Testnet $MANIFEST timed out after ${DURATION}s"
        else
            log_error "Testnet $MANIFEST failed after ${DURATION}s (exit code: $EXIT_CODE)"
        fi

        # Dump failure information
        log_warning "Dumping manifest content..."
        cat "$MANIFEST"
        
        log_warning "Dumping container logs..."
        "$RUNNER" -f "$MANIFEST" logs
        
        log_warning "Cleaning up failed testnet..."
        "$RUNNER" -f "$MANIFEST" cleanup
        
        FAILED+=("$MANIFEST")
    fi

    echo ""
done

# Summary
TOTAL_DURATION=$((SECONDS - TOTAL_START))
TOTAL_COUNT=$#
FAILED_COUNT=${#FAILED[@]}
SUCCESS_COUNT=${#SUCCESSFUL[@]}

echo "====== Test Summary ======"
echo "Total time: ${TOTAL_DURATION}s"
echo "Total tests: $TOTAL_COUNT"
echo "Successful: $SUCCESS_COUNT"
echo "Failed: $FAILED_COUNT"

if [[ $FAILED_COUNT -ne 0 ]]; then
    log_error "Failed testnets:"
    for MANIFEST in "${FAILED[@]}"; do
        echo "- $MANIFEST"
    done
    exit 1
else
    log_success "All testnets completed successfully"
    exit 0
fi
