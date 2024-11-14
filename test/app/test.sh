#!/usr/bin/env bash

# Strict mode
set -euo pipefail
IFS=$'\n\t'

# Configuration
readonly BARON_HOME="${HOME}/.baron_chain"
readonly LOG_DIR="${BARON_HOME}/logs"
readonly PID_DIR="${BARON_HOME}/pids"
readonly LOG_FILE="${LOG_DIR}/baron-chain.log"
readonly TEST_SCRIPT="test/app/kvstore_test.sh"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m'

# Initialize paths and environment
init_environment() {
    export PATH="${GOBIN}:${PATH}"
    
    # Create necessary directories
    mkdir -p "${LOG_DIR}" "${PID_DIR}"
    
    # Clean previous state
    rm -rf "${BARON_HOME}"/*
}

# Logging function
log() {
    local level=$1
    shift
    echo -e "${level}$(date '+%Y-%m-%d %H:%M:%S'): $*${NC}"
}

# Error handling
error_exit() {
    log "${RED}" "Error: $1"
    cleanup_processes
    exit 1
}

# Process management
start_process() {
    local name=$1
    local cmd=$2
    local pid_file="${PID_DIR}/${name}.pid"
    local log_file="${LOG_DIR}/${name}.log"
    
    log "${YELLOW}" "Starting ${name}..."
    
    # Start process with quantum-safe flags
    if [[ "$name" == "baron-chain" ]]; then
        $cmd --pqc-enabled --ai-optimization > "$log_file" 2>&1 &
    else
        $cmd > "$log_file" 2>&1 &
    fi
    
    local pid=$!
    echo "$pid" > "$pid_file"
    
    # Wait for process to start
    sleep 2
    if ! kill -0 "$pid" 2>/dev/null; then
        error_exit "${name} failed to start. Check ${log_file}"
    fi
    
    log "${GREEN}" "${name} started with PID ${pid}"
}

# Cleanup processes
cleanup_processes() {
    log "${YELLOW}" "Cleaning up processes..."
    
    for pid_file in "${PID_DIR}"/*.pid; do
        if [[ -f "$pid_file" ]]; then
            local pid
            pid=$(cat "$pid_file")
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid" || true
                log "${GREEN}" "Killed process ${pid}"
            fi
            rm "$pid_file"
        fi
    done
}

# Run KVStore test with cleanup
run_kvstore_test() {
    local test_name=$1
    local kvstore_first=${2:-true}
    
    trap cleanup_processes EXIT ERR
    
    init_environment
    
    # Initialize Baron Chain with quantum-safe config
    baron-chain init --quantum-safe
    
    if [[ "$kvstore_first" == true ]]; then
        # Start KVStore first
        start_process "kvstore" "baron-cli kvstore"
        sleep 2
        start_process "baron-chain" "baron-chain node"
    else
        # Start Baron Chain first
        start_process "baron-chain" "baron-chain node"
        sleep 2
        start_process "kvstore" "baron-cli kvstore"
    fi
    
    # Wait for services to be ready
    sleep 5
    
    log "${GREEN}" "Running test: ${test_name}"
    if ! bash "$TEST_SCRIPT" "$test_name"; then
        error_exit "Test failed: ${test_name}"
    fi
    
    log "${GREEN}" "Test completed successfully: ${test_name}"
    cleanup_processes
}

# Main execution
main() {
    case "${1:-all}" in
        "kvstore_first")
            run_kvstore_test "KVStore First Test" true
            ;;
        "chain_first")
            run_kvstore_test "Chain First Test" false
            ;;
        *)
            log "${YELLOW}" "Running all tests"
            run_kvstore_test "KVStore First Test" true
            echo ""
            run_kvstore_test "Chain First Test" false
            ;;
    esac
}

# Execute main with error handling
main "$@" || error_exit "Test execution failed"
