#!/usr/bin/env bash

# OSS-Fuzz Build Script for CometBFT
# This script compiles and configures fuzz tests for the CometBFT project.
# For more information, see: https://github.com/google/oss-fuzz/blob/master/projects/tendermint/build.sh

set -euo pipefail
IFS=$'\n\t'

# Configuration
readonly BASE_PKG="github.com/cometbft/cometbft/test/fuzz"
readonly FUZZ_TARGETS=(
    "mempool/v0:mempool_v0"
    "mempool/v1:mempool_v1"
    "p2p/addrbook:p2p_addrbook"
    "p2p/pex:p2p_pex"
    "p2p/secret_connection:p2p_secret_connection"
    "rpc/jsonrpc/server:rpc_jsonrpc_server"
)

# Log levels
readonly LOG_INFO="[\033[0;34mINFO\033[0m]"
readonly LOG_ERROR="[\033[0;31mERROR\033[0m]"
readonly LOG_SUCCESS="[\033[0;32mSUCCESS\033[0m]"

log_info() {
    echo -e "${LOG_INFO} $1"
}

log_error() {
    echo -e "${LOG_ERROR} $1" >&2
}

log_success() {
    echo -e "${LOG_SUCCESS} $1"
}

compile_fuzzer() {
    local pkg_path=$1
    local name=$2
    
    log_info "Compiling fuzzer: ${name}"
    
    if ! compile_go_fuzzer "${BASE_PKG}/${pkg_path}" Fuzz "${name}_fuzzer"; then
        log_error "Failed to compile fuzzer: ${name}"
        return 1
    fi
    
    log_success "Successfully compiled fuzzer: ${name}"
    return 0
}

main() {
    local exit_code=0
    local compiled_count=0
    local failed_count=0
    
    log_info "Starting CometBFT fuzzer compilation"
    log_info "Found ${#FUZZ_TARGETS[@]} fuzz targets to compile"
    
    for target in "${FUZZ_TARGETS[@]}"; do
        IFS=':' read -r pkg_path name <<< "$target"
        
        if compile_fuzzer "$pkg_path" "$name"; then
            ((compiled_count++))
        else
            ((failed_count++))
            exit_code=1
        fi
    done
    
    log_info "Compilation summary:"
    log_info "- Successfully compiled: ${compiled_count}"
    if [ $failed_count -gt 0 ]; then
        log_error "- Failed to compile: ${failed_count}"
    fi
    
    if [ $exit_code -eq 0 ]; then
        log_success "All fuzz targets compiled successfully"
    else
        log_error "Some fuzz targets failed to compile"
    fi
    
    return $exit_code
}

# Execute main function
main "$@"
