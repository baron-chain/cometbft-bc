#!/usr/bin/env bash

# Enable strict error handling
set -euo pipefail

# Configuration
readonly VERSION_FILE="../version/version.go"
readonly DOCKER_REPO="cometbft/cometbft"
readonly DEFAULT_TAGS=("latest")

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

# Prepare docker build arguments
prepare_build_args() {
    local tags=($1)
    local build_args=()
    
    for tag in "${tags[@]}"; do
        build_args+=(-t "${DOCKER_REPO}:${tag}")
    done
    
    echo "${build_args[@]}"
}

# Build images with confirmation
build_images() {
    local build_args=($1)
    local tag_list="${build_args[*]}"
    
    echo "==> Available tags: ${tag_list//-t/}"
    read -rp "==> Build docker images with these tags? [y/N] " response
    
    if [[ "${response,,}" =~ ^y(es)?$ ]]; then
        echo "==> Building Docker images..."
        docker build "${build_args[@]}" .
        echo "==> Build completed successfully"
    else
        echo "==> Build cancelled"
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

    # Generate tags and build arguments
    TAGS=$(generate_tags "$TAG")
    BUILD_ARGS=$(prepare_build_args "$TAGS")
    
    # Execute build
    build_images "$BUILD_ARGS"
}

main "$@"
