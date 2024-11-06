#!/usr/bin/env bash

# Test coverage script for CometBFT
# Runs tests with race detection and generates coverage report

set -euo pipefail
IFS=$'\n\t'

# Configuration
readonly COVERAGE_FILE="coverage.txt"
readonly PROFILE_FILE="profile.out"
readonly TIMEOUT="5m"
readonly PROJECT_PATH="github.com/cometbft/cometbft/..."

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    if [[ -f "${PROFILE_FILE}" ]]; then
        rm -f "${PROFILE_FILE}"
    fi
}

# Error handler
error_handler() {
    local line_no=$1
    local command=$2
    local error_code=$3
    echo -e "${RED}Error occurred in test coverage script${NC}"
    echo "Line: ${line_no}"
    echo "Command: ${command}"
    echo "Error code: ${error_code}"
}

# Progress indicator
progress() {
    local pkg=$1
    echo -e "${YELLOW}Testing package: ${pkg}${NC}"
}

# Initialize coverage file
init_coverage() {
    echo "mode: atomic" > "${COVERAGE_FILE}"
    echo -e "${GREEN}Initialized coverage file: ${COVERAGE_FILE}${NC}"
}

# Run tests for a package
test_package() {
    local pkg=$1
    progress "${pkg}"
    
    if ! go test -timeout "${TIMEOUT}" -race -coverprofile="${PROFILE_FILE}" -covermode=atomic "${pkg}"; then
        echo -e "${RED}Tests failed for package: ${pkg}${NC}"
        return 1
    fi
    
    if [[ -f "${PROFILE_FILE}" ]]; then
        tail -n +2 "${PROFILE_FILE}" >> "${COVERAGE_FILE}"
    fi
}

# Main execution
main() {
    # Set up error handling
    trap 'error_handler ${LINENO} "${BASH_COMMAND}" $?' ERR
    trap cleanup EXIT

    # Print Go version
    go version
    
    # Get all packages
    echo -e "${GREEN}Finding packages...${NC}"
    local packages
    packages=$(go list "${PROJECT_PATH}")
    
    # Initialize coverage file
    init_coverage
    
    # Count total packages
    local total_pkgs
    total_pkgs=$(echo "${packages}" | wc -l)
    echo -e "${GREEN}Found ${total_pkgs} packages to test${NC}"
    
    # Test counter
    local counter=0
    
    # Run tests for each package
    while IFS= read -r pkg; do
        ((counter++))
        echo -e "${GREEN}[$counter/$total_pkgs] Testing package${NC}"
        if ! test_package "${pkg}"; then
            echo -e "${RED}Testing failed for package: ${pkg}${NC}"
            exit 1
        fi
    done <<< "${packages}"
    
    # Final report
    echo -e "${GREEN}Coverage test completed successfully!${NC}"
    echo -e "${GREEN}Coverage file: ${COVERAGE_FILE}${NC}"
    
    # Optional: Generate HTML coverage report
    if command -v go-tool-cover &> /dev/null; then
        go tool cover -html="${COVERAGE_FILE}" -o coverage.html
        echo -e "${GREEN}HTML coverage report generated: coverage.html${NC}"
    fi
}

# Execute main function
main "$@"
