#!/usr/bin/env bash
set -euo pipefail

TAG=${TAG:-$(awk -F\" '/TMCoreSemVer =/ { print $2; exit }' ../version/version.go)}

if [[ -z "$TAG" ]]; then
    echo "Error: TAG not specified and couldn't be extracted from version.go" >&2
    exit 1
fi

TAG_NO_PATCH=${TAG%.*}
DOCKER_REPO="cometbft/cometbft"
TAGS=("latest" "$TAG" "$TAG_NO_PATCH")

read -rp "==> Push docker images with tags (${TAGS[*]})? [y/N] " response

if [[ "${response,,}" =~ ^y(es)?$ ]]; then
    for tag in "${TAGS[@]}"; do
        docker push "${DOCKER_REPO}:${tag}"
    done
fi
