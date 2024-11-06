#!/usr/bin/env bash

# Load Testing Script for CometBFT
# Executes performance testing with configurable parameters

set -euo pipefail
IFS=$'\n\t'

# Default configuration
readonly DEFAULT_CONNECTIONS=1
readonly DEFAULT_TIME=10
readonly DEFAULT_RATE=1000
readonly DEFAULT_SIZE=1024
readonly DEFAULT_METHOD="sync"
readonly DEFAULT_ENDPOINT="ws://localhost:26657/websocket"
readonly LOAD_BINARY="./build/load"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Help message
show_help() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Load testing script for CometBFT.

Options:
    -h, --help                  Show this help message
    -c, --connections NUM       Number of connections (default: ${DEFAULT_CONNECTIONS})
    -t, --time NUM             Test duration in seconds (default: ${DEFAULT_TIME})
    -r, --rate NUM             Transactions per second (default: ${DEFAULT_RATE})
    -s, --size NUM             Transaction size in bytes (default: ${DEFAULT_SIZE})
    -m, --method METHOD        Broadcast tx method [sync|async|commit] (default: ${DEFAULT_METHOD})
    -e, --endpoint URL         WebSocket endpoint (default: ${DEFAULT_ENDPOINT})
    -v, --verbose              Enable verbose output
    
Example:
    $(basename "$0") -c 2 -t 30 -r 2000 -s 2048
EOF
}

# Error handler
error_handler() {
    local line_no=$1
    local command=$2
    local error_code=$3
    echo -e "${RED}Error occurred in load test script${NC}"
    echo "Line: ${line_no}"
    echo "Command: ${command}"
    echo "Error code: ${error_code}"
    exit "${error_code}"
}

# Validate input parameters
validate_params() {
    if [[ $connections -lt 1 ]]; then
        echo -e "${RED}Error: Connections must be at least 1${NC}"
        exit 1
    fi
    if [[ $time -lt 1 ]]; then
        echo -e "${RED}Error: Time must be at least 1 second${NC}"
        exit 1
    fi
    if [[ $rate -lt 1 ]]; then
        echo -e "${RED}Error: Rate must be at least 1 tx/s${NC}"
        exit 1
    fi
    if [[ $size -lt 1 ]]; then
        echo -e "${RED}Error: Size must be at least 1 byte${NC}"
        exit 1
    fi
    case $method in
        sync|async|commit) ;;
        *)
            echo -e "${RED}Error: Invalid method. Must be sync, async, or commit${NC}"
            exit 1
            ;;
    esac
}

# Check prerequisites
check_prerequisites() {
    if [[ ! -f "${LOAD_BINARY}" ]]; then
        echo -e "${RED}Error: Load testing binary not found at ${LOAD_BINARY}${NC}"
        echo "Please build the binary first"
        exit 1
    fi

    # Check endpoint availability
    if ! curl --output /dev/null --silent --head --fail "${endpoint/ws/http}"; then
        echo -e "${YELLOW}Warning: Endpoint ${endpoint} might not be accessible${NC}"
    fi
}

# Initialize test configuration
init_config() {
    connections=${DEFAULT_CONNECTIONS}
    time=${DEFAULT_TIME}
    rate=${DEFAULT_RATE}
    size=${DEFAULT_SIZE}
    method=${DEFAULT_METHOD}
    endpoint=${DEFAULT_ENDPOINT}
    verbose=false
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -c|--connections)
                connections=$2
                shift 2
                ;;
            -t|--time)
                time=$2
                shift 2
                ;;
            -r|--rate)
                rate=$2
                shift 2
                ;;
            -s|--size)
                size=$2
                shift 2
                ;;
            -m|--method)
                method=$2
                shift 2
                ;;
            -e|--endpoint)
                endpoint=$2
                shift 2
                ;;
            -v|--verbose)
                verbose=true
                shift
                ;;
            *)
                echo -e "${RED}Unknown option: $1${NC}"
                show_help
                exit 1
                ;;
        esac
    done
}

# Execute load test
run_load_test() {
    echo -e "${GREEN}Starting load test with following parameters:${NC}"
    echo "Connections: $connections"
    echo "Duration: $time seconds"
    echo "Rate: $rate tx/s"
    echo "Size: $size bytes"
    echo "Method: $method"
    echo "Endpoint: $endpoint"

    if [[ "${verbose}" == true ]]; then
        set -x
    fi

    ${LOAD_BINARY} \
        -c "${connections}" \
        -T "${time}" \
        -r "${rate}" \
        -s "${size}" \
        --broadcast-tx-method "${method}" \
        --endpoints "${endpoint}"

    if [[ "${verbose}" == true ]]; then
        set +x
    fi
}

# Main execution
main() {
    # Set up error handling
    trap 'error_handler ${LINENO} "${BASH_COMMAND}" $?' ERR

    # Initialize configuration
    init_config

    # Parse command line arguments
    parse_args "$@"

    # Validate parameters
    validate_params

    # Check prerequisites
    check_prerequisites

    # Run load test
    run_load_test

    echo -e "${GREEN}Load test completed successfully!${NC}"
}

# Execute main function
main "$@"
