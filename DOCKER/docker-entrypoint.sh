#!/bin/bash
set -euo pipefail

CONFIG_DIR="${CMTHOME:?}/config"
CONFIG_FILE="$CONFIG_DIR/config.toml"
GENESIS_FILE="$CONFIG_DIR/genesis.json"

if [[ ! -d "$CONFIG_DIR" ]]; then
    echo "Initializing CometBFT with default configuration..."
    cometbft init

    # Update config.toml settings
    sed -i \
        -e "s/^proxy_app\s*=.*/proxy_app = \"${PROXY_APP:?}\"/" \
        -e "s/^moniker\s*=.*/moniker = \"${MONIKER:?}\"/" \
        -e 's/^addr_book_strict\s*=.*/addr_book_strict = false/' \
        -e 's/^timeout_commit\s*=.*/timeout_commit = "500ms"/' \
        -e 's/^index_all_tags\s*=.*/index_all_tags = true/' \
        -e 's,^laddr = "tcp://127.0.0.1:26657",laddr = "tcp://0.0.0.0:26657",' \
        -e 's/^prometheus\s*=.*/prometheus = true/' \
        "$CONFIG_FILE"

    # Update genesis.json
    jq --arg chain_id "${CHAIN_ID:?}" \
       '.chain_id = $chain_id | .consensus_params.block.time_iota_ms = "500"' \
       "$GENESIS_FILE" > "${GENESIS_FILE}.tmp" && \
    mv "${GENESIS_FILE}.tmp" "$GENESIS_FILE"
fi

exec cometbft "$@"
