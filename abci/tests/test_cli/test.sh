#!/usr/bin/env bash

# Strict mode settings
set -euo pipefail
IFS=$'\n\t'

# Script constants
readonly TIMEOUT_SECONDS=10
readonly LOG_LEVEL="error"
readonly WAIT_TIME=2
readonly SUCCESS_MSG="\e[32mPASS\e[0m"
readonly ERROR_MSG="\e[31mERROR\e[0m"

# Initialize variables
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TEMP_DIR="/tmp/abci-tests-$$"
PIDS=()

# Logging functions
log_info() {
    printf "\e[34m[INFO]\e[0m %s\n" "$1"
}

log_error() {
    printf "\e[31m[ERROR]\e[0m %s\n" "$1" >&2
}

log_success() {
    printf "\e[32m[SUCCESS]\e[0m %s\n" "$1"
}

# Cleanup function
cleanup() {
    local exit_code=$?
    log_info "Performing cleanup..."

    # Kill any remaining processes
    for pid in "${PIDS[@]:-}"; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
    done

    # Remove temporary files
    if [[ -d "${TEMP_DIR}" ]]; then
        rm -rf "${TEMP_DIR}"
    fi

    # Remove any .out.new files
    find "${ROOT_DIR}" -name "*.out.new" -delete

    if [[ $exit_code -eq 0 ]]; then
        echo -e "${SUCCESS_MSG}"
    else
        echo -e "${ERROR_MSG} Test script failed"
    fi

    exit "$exit_code"
}

# Set up trap for cleanup
trap cleanup EXIT INT TERM

# Check required commands
check_dependencies() {
    local deps=(abci-cli shasum diff)
    local missing=()

    for cmd in "${deps[@]}"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing+=("$cmd")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required dependencies: ${missing[*]}"
        exit 1
    fi
}

# Verify the application starts successfully
wait_for_app() {
    local app_name="$1"
    local pid="$2"
    local timeout="$TIMEOUT_SECONDS"

    while [[ $timeout -gt 0 ]]; do
        if ! kill -0 "$pid" 2>/dev/null; then
            log_error "$app_name failed to start"
            exit 1
        fi
        if lsof -i :26658 >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
        ((timeout--))
    done

    log_error "$app_name failed to start within $TIMEOUT_SECONDS seconds"
    exit 1
}

# Test a single example
test_example() {
    local number="$1"
    local input="$2"
    local app_cmd="$3"
    local app_args="${4:-}"
    local app_name
    app_name=$(basename "$app_cmd")

    log_info "Testing Example ${number}: ${app_cmd} ${app_args}"

    # Validate input file
    if [[ ! -f "${input}" ]]; then
        log_error "Input file not found: ${input}"
        exit 1
    }

    # Start the application
    ${app_cmd} ${app_args} &>/dev/null &
    local pid=$!
    PIDS+=("$pid")

    # Wait for app to start
    wait_for_app "$app_name" "$pid"

    # Create output directory if it doesn't exist
    mkdir -p "${TEMP_DIR}"
    local temp_output="${TEMP_DIR}/$(basename "${input}").out.new"

    # Run the test
    if ! abci-cli \
        --log_level="${LOG_LEVEL}" \
        --verbose \
        batch < "${input}" > "${temp_output}"; then
        log_error "abci-cli command failed"
        exit 1
    fi

    # Compare outputs
    local expected_output="${input}.out"
    if ! cmp -s "${expected_output}" "${temp_output}"; then
        log_error "Example ${number} failed: Output mismatch"
        echo "Got:"
        cat "${temp_output}"
        echo "Expected:"
        cat "${expected_output}"
        echo "Diff:"
        diff "${expected_output}" "${temp_output}" || true
        exit 1
    fi

    # Cleanup
    kill "$pid" || true
    wait "$pid" 2>/dev/null || true
    rm -f "${temp_output}"

    log_success "Example ${number} passed"
}

main() {
    # Change to root directory
    cd "${ROOT_DIR}" || exit 1

    # Check dependencies
    check_dependencies

    # Ensure GOBIN is in PATH
    if [[ -n "${GOBIN:-}" ]]; then
        export PATH="$GOBIN:$PATH"
    fi

    # Run tests
    test_example 1 "tests/test_cli/ex1.abci" "abci-cli" "kvstore"
    test_example 2 "tests/test_cli/ex2.abci" "abci-cli" "kvstore"

    log_success "All tests passed!"
}

# Run main function
main "$@"
