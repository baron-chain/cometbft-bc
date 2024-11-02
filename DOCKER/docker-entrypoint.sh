#!/bin/bash

# Enable strict error handling
set -euo pipefail

# Configuration
readonly CMTHOME="${CMTHOME:?}"
readonly CONFIG_DIR="$CMTHOME/config"
readonly CONFIG_FILE="$CONFIG_DIR/config.toml"
readonly GENESIS_FILE="$CONFIG_DIR/genesis.json"
readonly GENESIS_TEMP="${GENESIS_FILE}.tmp"

# Default configuration values
readonly DEFAULT_TIMEOUT="500ms"
readonly DEFAULT_LISTEN_ADDR="tcp://0.0.0.0:26657"

update_config() {
    local config_file=$1
    local proxy_app="${PROXY_APP:?}"
    local moniker="${MONIKER:?}"
    
    sed -i \
        -e "s/^proxy_app\s*=.*/proxy_app = \"$proxy_app\"/" \
        -e "s/^moniker\s*=.*/moniker = \"$moniker\"/" \
        -e 's/^addr_book_strict\s*=.*/addr_book_strict = false/' \
        -e "s/^timeout_commit\s*=.*/timeout_commit = \"$DEFAULT_TIMEOUT\"/" \
        -e 's/^index_all_tags\s*=.*/index_all_tags = true/' \
        -e "s,^laddr = \"tcp://127.0.0.1:26657\",laddr = \"$DEFAULT_LISTEN_ADDR\"," \
        -e 's/^prometheus\s*=.*/prometheus = true/' \
        "$config_file"
}

update_genesis() {
    local genesis_file=$1
    local temp_file=$2
    local chain_id="${CHAIN_ID:?}"
    
    jq --arg chain_id "$chain_id" \
       --arg time_iota "${DEFAULT_TIMEOUT%ms}" \
       '.chain_id = $chain_id | .consensus_params.block.time_iota_ms = $time_iota' \
       "$genesis_file" > "$temp_file" && \
    mv "$temp_file" "$genesis_file"
}

initialize_cometbft() {
    if [[ ! -d "$CONFIG_DIR" ]]; then
        echo "Initializing CometBFT with default configuration..."
        cometbft init
        
        echo "Updating configuration..."
        update_config "$CONFIG_FILE"
        
        echo "Updating genesis file..."
        update_genesis "$GENESIS_FILE" "$GENESIS_TEMP"
    else
        echo "Configuration directory already exists, skipping initialization"
    fi
}

main() {
    initialize_cometbft
    exec cometbft "$@"
}

main "$@"
