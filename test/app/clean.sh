#!/usr/bin/env bash

# Exit on error, undefined variables, and pipe failures
set -euo pipefail

# Color setup for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Default paths
BARON_HOME="${HOME}/.baron_chain"
BARON_APP="${HOME}/.baron_chain_app"
PID_FILE="/tmp/baron-chain.pid"

# Logging function
log() {
    local level=$1
    shift
    echo -e "${level}$(date '+%Y-%m-%d %H:%M:%S'): $*${NC}"
}

# Cleanup function
cleanup() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        log "${RED}" "Cleanup failed with exit code: $exit_code"
    fi
    exit $exit_code
}

trap cleanup EXIT

# Check if processes are running
check_process() {
    local process=$1
    if pgrep -x "$process" > /dev/null; then
        return 0
    else
        return 1
    fi
}

# Gracefully stop processes
stop_process() {
    local process=$1
    local timeout=10

    if check_process "$process"; then
        log "${YELLOW}" "Stopping $process..."
        killall -s SIGTERM "$process" 2>/dev/null || true
        
        # Wait for process to stop
        while [ $timeout -gt 0 ] && check_process "$process"; do
            sleep 1
            ((timeout--))
        done

        # Force kill if still running
        if check_process "$process"; then
            log "${RED}" "$process didn't stop gracefully, force killing..."
            killall -s SIGKILL "$process" 2>/dev/null || true
        fi
    else
        log "${GREEN}" "$process is not running"
    fi
}

main() {
    log "${YELLOW}" "Starting Baron Chain cleanup..."

    # Stop baron-chain process
    stop_process "baron-chain"

    # Stop ABCI application
    stop_process "baron-cli"

    # Remove data directories
    if [ -d "$BARON_HOME" ]; then
        log "${YELLOW}" "Removing Baron Chain home directory..."
        rm -rf "$BARON_HOME"
    fi

    if [ -d "$BARON_APP" ]; then
        log "${YELLOW}" "Removing Baron Chain app directory..."
        rm -rf "$BARON_APP"
    fi

    # Remove PID file if exists
    if [ -f "$PID_FILE" ]; then
        rm -f "$PID_FILE"
    fi

    # Remove any quantum-safe temp files
    rm -rf /tmp/baron-pqc-* 2>/dev/null || true
    
    # Remove AI model cache
    rm -rf /tmp/baron-ai-cache-* 2>/dev/null || true

    log "${GREEN}" "Baron Chain cleanup completed successfully"
}

# Execute main function
main
