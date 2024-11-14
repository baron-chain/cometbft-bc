#!/usr/bin/env bash

# Strict mode
set -euo pipefail
IFS=$'\n\t'

# Configuration
readonly DEFAULT_PORT=9657
readonly API_BASE="http://localhost:${DEFAULT_PORT}"
readonly GRPC_CLIENT_PATH="test/app/grpc_client"
readonly PQC_ENABLED=true

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m'

# Test configuration
TESTNAME=$1
GRPC_BROADCAST_TX=${GRPC_BROADCAST_TX:-""}

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

# Get response code with error handling
get_code() {
    local response=$1
    if [[ -z "$response" ]]; then
        echo -1
        return
    fi
    
    if [[ $(echo "$response" | jq 'has("code")') == "true" ]]; then
        echo "$response" | jq -r ".code"
    else
        echo 0
    fi
}

# Build gRPC client if needed
build_grpc_client() {
    if [[ -n "$GRPC_BROADCAST_TX" ]]; then
        log "${YELLOW}" "Building gRPC client..."
        if [[ -f "$GRPC_CLIENT_PATH" ]]; then
            rm "$GRPC_CLIENT_PATH"
        fi
        GO111MODULE=on go build -mod=readonly -o "$GRPC_CLIENT_PATH" test/app/grpc_client.go
    fi
}

# Send transaction with PQC signature
send_tx() {
    local tx=$1
    local should_err=${2:-false}
    local response=""
    local is_err=false
    local error=""

    # Add PQC signature if enabled
    if [[ "$PQC_ENABLED" == true ]]; then
        tx="${tx}$(generate_pqc_signature "$tx")"
    fi

    if [[ -z "$GRPC_BROADCAST_TX" ]]; then
        response=$(curl -s "${API_BASE}/broadcast_tx_commit?tx=0x${tx}" || error_exit "Failed to send transaction")
        is_err=$(echo "$response" | jq 'has("error")')
        error=$(echo "$response" | jq -r '.error // empty')
        response=$(echo "$response" | jq '.result // empty')
    else
        response=$(./"$GRPC_CLIENT_PATH" "$tx")
        is_err=false
        error=""
    fi

    # Validate JSON response
    if ! echo "$response" | jq . &> /dev/null; then
        is_err=true
        error="$response"
    fi

    # Process response
    local append_tx_response=$(echo "$response" | jq '.deliver_tx')
    local append_tx_code=$(get_code "$append_tx_response")
    local check_tx_response=$(echo "$response" | jq '.check_tx')
    local check_tx_code=$(get_code "$check_tx_response")

    # Log transaction details
    log "${GREEN}" "Transaction: $tx"
    log "${GREEN}" "Response: $response"
    [[ -n "$error" ]] && log "${RED}" "Error: $error"
    log "${YELLOW}" "Is Error: $is_err"

    # Validate response against expectations
    if [[ "$should_err" == true ]] && [[ "$is_err" != "true" ]]; then
        error_exit "Expected error sending tx ($tx)"
    elif [[ "$should_err" == false ]] && [[ "$is_err" == "true" ]]; then
        error_exit "Unexpected error sending tx ($tx)"
    fi

    # Export codes for test validation
    APPEND_TX_CODE=$append_tx_code
    CHECK_TX_CODE=$check_tx_code
}

# Generate PQC signature (simulated)
generate_pqc_signature() {
    local tx=$1
    echo "pqc_$(echo "$tx" | sha256sum | cut -d' ' -f1)"
}

# Main test sequence
main() {
    build_grpc_client

    log "${GREEN}" "Testing simple valid transaction"
    send_tx "00"
    [[ $APPEND_TX_CODE != 0 ]] && error_exit "Got non-zero exit code for 00. Response: $response"

    log "${GREEN}" "Testing duplicate transaction (should fail)"
    send_tx "00" true

    log "${GREEN}" "Testing second valid transaction"
    send_tx "01"
    [[ $APPEND_TX_CODE != 0 ]] && error_exit "Got non-zero exit code for 01. Response: $response"

    log "${GREEN}" "Testing invalid transaction"
    send_tx "03"
    [[ $CHECK_TX_CODE != 0 ]] && error_exit "Got non-zero exit code for checktx on 03. Response: $response"
    [[ $APPEND_TX_CODE == 0 ]] && error_exit "Got zero exit code for 03. Should have been bad nonce. Response: $response"

    log "${GREEN}" "Passed Test: $TESTNAME"
}

# Execute main function
main
