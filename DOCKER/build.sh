#!/usr/bin/env bash
set -euo pipefail

# Configuration
readonly VERSION_FILE="../version/version.go"
readonly DOCKER_REPO="baronchain/node"
readonly BUILD_DATE=$(date -u +'%Y-%m-%d')
readonly GIT_COMMIT=$(git rev-parse --short HEAD)

get_version_tag() {
    local version
    version=$(awk -F\" '/BaronChainVersion =/ { print $2; exit }' "$VERSION_FILE")
    [[ -z "$version" ]] && version="0.1.0"
    echo "$version"
}

generate_tags() {
    local version=$1
    local major_minor=${version%.*}
    cat << EOF
latest
${version}
${major_minor}
${GIT_COMMIT}
EOF
}

prepare_build_args() {
    local tags=($1)
    local build_args=(
        --build-arg BUILD_DATE="$BUILD_DATE"
        --build-arg BUILD_VERSION="$version"
        --build-arg GIT_COMMIT="$GIT_COMMIT"
        --label "org.opencontainers.image.created=$BUILD_DATE"
        --label "org.opencontainers.image.version=$version"
        --label "org.opencontainers.image.revision=$GIT_COMMIT"
    )
    
    for tag in "${tags[@]}"; do
        build_args+=(-t "${DOCKER_REPO}:${tag}")
    done
    
    echo "${build_args[@]}"
}

build_images() {
    local build_args=($1)
    local tag_list="${build_args[*]}"
    
    echo "Baron Chain Docker Image Builder"
    echo "================================"
    echo "Version: ${version}"
    echo "Commit:  ${GIT_COMMIT}"
    echo "Date:    ${BUILD_DATE}"
    echo "Tags:    ${tag_list//-t/}"
    echo "================================"
    
    read -rp "Build images with these tags? [y/N] " response
    
    if [[ "${response,,}" =~ ^y(es)?$ ]]; then
        echo "Building Baron Chain images..."
        
        if ! docker build "${build_args[@]}" .; then
            echo "Error: Build failed" >&2
            exit 1
        fi
        
        echo "Build completed successfully"
    else
        echo "Build cancelled"
        exit 0
    fi
}

main() {
    local version=${TAG:-$(get_version_tag)}
    
    if [[ -z "$version" ]]; then
        echo "Error: Version tag not found" >&2
        echo "Please specify TAG environment variable or check $VERSION_FILE" >&2
        exit 1
    fi
    
    local tags
    tags=$(generate_tags "$version")
    
    local build_args
    build_args=$(prepare_build_args "$tags")
    
    build_images "$build_args"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
