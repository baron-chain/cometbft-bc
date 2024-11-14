#!/usr/bin/env bash

# Strict mode
set -euo pipefail
IFS=$'\n\t'

# Configuration
readonly API_HOST="127.0.0.1"
readonly API_PORT="9657"
readonly API_BASE="http://${API_HOST}:${API_PORT}"
readonly CLI_NAME="baron-cli"
readonly TEST_KEY="abcd"
readonly TEST_VALUE="dcba"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m'

# Get test name from args
TESTNAME=$1

# Logging function
log() {
    local level=$1
    shift
    echo -e "${level}$(date '+%Y-%m-%d %H:%M:%S'): $*${NC}"
}

# Error handling
error_exit() {
    log "${RED}" "Error: $1"
    exit 1
}

# Convert string to hex with PQC signature
to_hex_with_pqc() {
    local input=$1
    local hex_value
    hex_value=$(echo -n "$input" | hexdump -ve '1/1 "%.2X"')
    
    # Add PQC signature simulation
    local pqc_sig="504843" # "PQC" in hex
    echo "0x${hex_value}${pqc_sig}"
}

# Validate response
validate_response() {
    local response=$1
    local expected=$2
    local should_exist=${3:-true}
    
    if [[ "$should_exist" == true ]]; then
        if ! echo "$response" | grep -q "$expected"; then
            error_exit "Failed to find '$expected' in response: $response"
        fi
    else
        if echo "$response" | grep -q "$expected"; then
            error_exit "Found unexpected '$expected' in response: $response"
        fi
    fi
}

# Test store operation
test_store() {
    log "${GREEN}" "Testing key-value store operation"
    local tx_data
    tx_data=$(to_hex_with_pqc "${TEST_KEY}=${TEST_VALUE}")
    
    local response
    response=$(curl -s "${API_BASE}/broadcast_tx_commit?tx=${tx_data}")
    
    if [[ $(echo "$response" | jq -r '.result.deliver_tx.code // 1') != "0" ]]; then
        error_exit "Failed to store key-value pair. Response: $response"
    }
    
    log "${GREEN}" "Successfully stored key-value pair"
}

# Test CLI query
test_cli_query() {
    log "${GREEN}" "Testing CLI query functionality"
    
    # Test key lookup
    local key_response
    key_response=$($CLI_NAME query "$TEST_KEY" 2>&1)
    validate_response "$key_response" "$TEST_VALUE"
    
    # Test value lookup (should not exist)
    local value_response
    value_response=$($CLI_NAME query "$TEST_VALUE" 2>&1)
    validate_response "$value_response" "value: $TEST_VALUE" false
    
    log "${GREEN}" "CLI query tests passed"
}

# Test API query
test_api_query() {
    log "${GREEN}" "Testing API query functionality"
    
    # Test key lookup
    local key_query
    key_query=$(to_hex_with_pqc "$TEST_KEY")
    local key_response
    key_response=$(curl -s "${API_BASE}/abci_query?path=\"\"&data=${key_query}&prove=false")
    validate_response "$(echo "$key_response" | jq -r '.result.response.log')" "exists"
    
    # Test value lookup (should not exist)
    local value_query
    value_query=$(to_hex_with_pqc "$TEST_VALUE")
    local value_response
    value_response=$(curl -s "${API_BASE}/abci_query?path=\"\"&data=${value_query}&prove=false")
    validate_response "$(echo "$value_response" | jq -r '.result.response.log')" "exists" false
    
    log "${GREEN}" "API query tests passed"
}

# Check required commands
check_dependencies() {
    local deps=("curl" "jq" "hexdump" "$CLI_NAME")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            error_exit "Required command not found: $dep"
        fi
    done
}

# Main function
main() {
    log "${YELLOW}" "Starting KVStore tests: $TESTNAME"
    
    check_dependencies
    
    # Run tests
    test_store
    test_cli_query
    test_api_query
    
    log "${GREEN}" "Passed Test: $TESTNAME"
}

# Execute main with error handling
main "$@" || error_exit "Test execution failed"
