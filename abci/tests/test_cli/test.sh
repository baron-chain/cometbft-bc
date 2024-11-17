#!/usr/bin/env bash

# Strict mode settings
set -euo pipefail
IFS=$'\n\t'

# Baron Chain test constants
readonly BC_TIMEOUT=10
readonly BC_LOG_LEVEL="error"
readonly BC_WAIT_TIME=2
readonly BC_SUCCESS="\e[32mBaron Chain Test PASSED\e[0m"
readonly BC_ERROR="\e[31mBaron Chain Test FAILED\e[0m"

# Environment setup
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TEMP_DIR="/tmp/baron-chain-tests-$$"
PIDS=()

# Logging utilities
bc_log_info() {
    printf "\e[34m[BARON]\e[0m %s\n" "$1"
}

bc_log_error() {
    printf "\e[31m[BARON ERROR]\e[0m %s\n" "$1" >&2
}

bc_log_success() {
    printf "\e[32m[BARON SUCCESS]\e[0m %s\n" "$1"
}

# Cleanup handler
bc_cleanup() {
    local exit_code=$?
    bc_log_info "Cleaning up Baron Chain test environment..."

    # Terminate test processes
    for pid in "${PIDS[@]:-}"; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
    done

    # Clean test artifacts
    if [[ -d "${TEMP_DIR}" ]]; then
        rm -rf "${TEMP_DIR}"
    fi

    # Remove test outputs
    find "${ROOT_DIR}" -name "*.out.new" -delete

    if [[ $exit_code -eq 0 ]]; then
        echo -e "${BC_SUCCESS}"
    else
        echo -e "${BC_ERROR}"
    fi

    exit "$exit_code"
}

trap bc_cleanup EXIT INT TERM

# Dependency verification
bc_check_deps() {
    local deps=(baron-cli shasum diff)
    local missing=()

    for cmd in "${deps[@]}"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing+=("$cmd")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        bc_log_error "Missing Baron Chain dependencies: ${missing[*]}"
        exit 1
    fi
}

# Baron Chain node readiness check
bc_wait_for_node() {
    local node_name="$1"
    local pid="$2"
    local timeout="$BC_TIMEOUT"

    while [[ $timeout -gt 0 ]]; do
        if ! kill -0 "$pid" 2>/dev/null; then
            bc_log_error "Baron Chain node $node_name failed to start"
            exit 1
        fi
        if lsof -i :26658 >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
        ((timeout--))
    done

    bc_log_error "Baron Chain node $node_name startup timeout after $BC_TIMEOUT seconds"
    exit 1
}

# Test execution
bc_run_test() {
    local test_num="$1"
    local input="$2"
    local node_cmd="$3"
    local node_args="${4:-}"
    local node_name
    node_name=$(basename "$node_cmd")

    bc_log_info "Running Baron Chain Test ${test_num}: ${node_cmd} ${node_args}"

    # Validate test input
    if [[ ! -f "${input}" ]]; then
        bc_log_error "Baron Chain test file not found: ${input}"
        exit 1
    }

    # Launch node
    ${node_cmd} ${node_args} &>/dev/null &
    local pid=$!
    PIDS+=("$pid")

    # Verify node startup
    bc_wait_for_node "$node_name" "$pid"

    # Setup test output
    mkdir -p "${TEMP_DIR}"
    local test_output="${TEMP_DIR}/$(basename "${input}").out.new"

    # Execute test
    if ! baron-cli \
        --log_level="${BC_LOG_LEVEL}" \
        --verbose \
        batch < "${input}" > "${test_output}"; then
        bc_log_error "Baron Chain CLI test failed"
        exit 1
    }

    # Verify test results
    local expected="${input}.out"
    if ! cmp -s "${expected}" "${test_output}"; then
        bc_log_error "Baron Chain Test ${test_num} failed: Output mismatch"
        echo "Actual Output:"
        cat "${test_output}"
        echo "Expected Output:"
        cat "${expected}"
        echo "Differences:"
        diff "${expected}" "${test_output}" || true
        exit 1
    }

    # Cleanup test
    kill "$pid" || true
    wait "$pid" 2>/dev/null || true
    rm -f "${test_output}"

    bc_log_success "Baron Chain Test ${test_num} completed successfully"
}

main() {
    cd "${ROOT_DIR}" || exit 1
    bc_check_deps

    # Ensure Baron Chain binaries are available
    if [[ -n "${GOBIN:-}" ]]; then
        export PATH="$GOBIN:$PATH"
    fi

    # Execute Baron Chain test suite
    bc_run_test 1 "tests/baron_chain/quantum_test.abci" "baron-cli" "kvstore"
    bc_run_test 2 "tests/baron_chain/ai_routing_test.abci" "baron-cli" "kvstore"

    bc_log_success "Baron Chain test suite completed successfully!"
}

main "$@"
