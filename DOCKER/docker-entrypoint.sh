#!/bin/bash
set -euo pipefail

# Configuration
readonly BARONHOME="${BARONHOME:-/baronchain}"
readonly CONFIG_DIR="$BARONHOME/config"
readonly CONFIG_FILE="$CONFIG_DIR/config.toml"
readonly GENESIS_FILE="$CONFIG_DIR/genesis.json"
readonly GENESIS_TEMP="${GENESIS_FILE}.tmp"

# Network defaults
readonly DEFAULT_TIMEOUT="1s"
readonly DEFAULT_LISTEN_ADDR="tcp://0.0.0.0:26657"
readonly DEFAULT_P2P_ADDR="tcp://0.0.0.0:26656"
readonly DEFAULT_METRICS_ADDR="tcp://0.0.0.0:26660"

update_config() {
    local config_file=$1
    local proxy_app="${PROXY_APP:-kvstore}"
    local moniker="${MONIKER:-baronnode}"
    
    sed -i \
        -e "s/^proxy_app\s*=.*/proxy_app = \"$proxy_app\"/" \
        -e "s/^moniker\s*=.*/moniker = \"$moniker\"/" \
        -e 's/^addr_book_strict\s*=.*/addr_book_strict = false/' \
        -e "s/^timeout_commit\s*=.*/timeout_commit = \"$DEFAULT_TIMEOUT\"/" \
        -e 's/^index_all_tags\s*=.*/index_all_tags = true/' \
        -e "s,^laddr = \"tcp://127.0.0.1:26657\",laddr = \"$DEFAULT_LISTEN_ADDR\"," \
        -e "s,^laddr = \"tcp://127.0.0.1:26656\",laddr = \"$DEFAULT_P2P_ADDR\"," \
        -e "s,^prometheus_listen_addr = \".*\",prometheus_listen_addr = \"$DEFAULT_METRICS_ADDR\"," \
        -e 's/^prometheus\s*=.*/prometheus = true/' \
        -e 's/^log_level\s*=.*/log_level = "info"/' \
        -e 's/^create_empty_blocks\s*=.*/create_empty_blocks = true/' \
        "$config_file"
}

update_genesis() {
    local genesis_file=$1
    local temp_file=$2
    local chain_id="${CHAIN_ID:-baronchain}"
    
    jq --arg chain_id "$chain_id" \
       --arg time_iota "${DEFAULT_TIMEOUT%s}000" \
       --arg max_bytes "22020096" \
       --arg max_gas "-1" \
       '.chain_id = $chain_id | 
        .consensus_params.block.time_iota_ms = ($time_iota | tonumber) |
        .consensus_params.block.max_bytes = ($max_bytes | tonumber) |
        .consensus_params.block.max_gas = ($max_gas | tonumber)' \
       "$genesis_file" > "$temp_file" && \
    mv "$temp_file" "$genesis_file"
}

initialize_node() {
    if [[ ! -d "$CONFIG_DIR" ]]; then
        echo "Initializing Baron Chain node..."
        baronchain init
        
        echo "Updating node configuration..."
        update_config "$CONFIG_FILE"
        
        echo "Updating genesis configuration..."
        update_genesis "$GENESIS_FILE" "$GENESIS_TEMP"
        
        echo "Node initialization completed"
    else
        echo "Configuration exists, skipping initialization"
    fi
}

check_prerequisites() {
    command -v jq >/dev/null 2>&1 || { echo "Error: jq is required but not installed" >&2; exit 1; }
    command -v baronchain >/dev/null 2>&1 || { echo "Error: baronchain binary not found" >&2; exit 1; }
}

main() {
    check_prerequisites
    
    # Handle initialization
    initialize_node
    
    # Start the node
    echo "Starting Baron Chain node..."
    exec baronchain "$@"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
