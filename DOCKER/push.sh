#!/usr/bin/env bash

# Enable strict error handling
set -euo pipefail

# Configuration
VERSION_FILE="../version/version.go"
DOCKER_REPO="cometbft/cometbft"
DEFAULT_TAGS=("latest")

# Extract version tag from version.go
get_version_tag() {
    local version
    version=$(awk -F\" '/TMCoreSemVer =/ { print $2; exit }' "$VERSION_FILE")
    echo "$version"
}

# Generate all required tags
generate_tags() {
    local full_version=$1
    local major_minor=${full_version%.*}
    echo -e "latest\n${full_version}\n${major_minor}"
}

# Push images with confirmation
push_images() {
    local tags=($1)
    local tag_list="${tags[*]}"
    
    echo "==> Available tags: $tag_list"
    read -rp "==> Push docker images with these tags? [y/N] " response
    
    if [[ "${response,,}" =~ ^y(es)?$ ]]; then
        for tag in "${tags[@]}"; do
            echo "==> Pushing ${DOCKER_REPO}:${tag}"
            docker push "${DOCKER_REPO}:${tag}"
        done
        echo "==> All images pushed successfully"
    else
        echo "==> Operation cancelled"
        exit 0
    fi
}

main() {
    # Get version tag (from environment or version file)
    TAG=${TAG:-$(get_version_tag)}
    
    if [[ -z "$TAG" ]]; then
        echo "Error: Version tag not found. Specify TAG environment variable or check $VERSION_FILE" >&2
        exit 1
    fi

    # Generate and push tags
    TAGS=$(generate_tags "$TAG")
    push_images "$TAGS"
}

main "$@"
