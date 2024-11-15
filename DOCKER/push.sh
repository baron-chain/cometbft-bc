#!/usr/bin/env bash
set -euo pipefail

# Configuration
readonly VERSION_FILE="../version/version.go"
readonly DOCKER_REPO="baronchain/node"
readonly DOCKER_REGISTRY="${DOCKER_REGISTRY:-docker.io}"
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

docker_login() {
    if [[ -n "${DOCKER_USERNAME:-}" ]] && [[ -n "${DOCKER_PASSWORD:-}" ]]; then
        echo "Logging into Docker registry..."
        echo "${DOCKER_PASSWORD}" | docker login --username "${DOCKER_USERNAME}" --password-stdin "${DOCKER_REGISTRY}"
    fi
}

push_images() {
    local tags=($1)
    local tag_list="${tags[*]}"
    
    echo "Baron Chain Image Push Tool"
    echo "=========================="
    echo "Registry: ${DOCKER_REGISTRY}"
    echo "Repository: ${DOCKER_REPO}"
    echo "Version: ${version}"
    echo "Tags: ${tag_list}"
    echo "=========================="
    
    read -rp "Push images with these tags? [y/N] " response
    
    if [[ "${response,,}" =~ ^y(es)?$ ]]; then
        local failed_tags=()
        
        for tag in "${tags[@]}"; do
            echo "Pushing ${DOCKER_REPO}:${tag}..."
            if ! docker push "${DOCKER_REPO}:${tag}"; then
                failed_tags+=("${tag}")
                echo "Failed to push ${tag}"
            fi
        done
        
        if [[ ${#failed_tags[@]} -eq 0 ]]; then
            echo "All images pushed successfully"
        else
            echo "Failed to push tags: ${failed_tags[*]}" >&2
            exit 1
        fi
    else
        echo "Operation cancelled"
        exit 0
    fi
}

cleanup() {
    if [[ -n "${DOCKER_USERNAME:-}" ]]; then
        echo "Logging out from Docker registry..."
        docker logout "${DOCKER_REGISTRY}" || true
    fi
}

main() {
    local version=${TAG:-$(get_version_tag)}
    
    if [[ -z "$version" ]]; then
        echo "Error: Version tag not found" >&2
        echo "Please specify TAG environment variable or check $VERSION_FILE" >&2
        exit 1
    fi
    
    trap cleanup EXIT
    
    docker_login
    
    local tags
    tags=$(generate_tags "$version")
    
    push_images "$tags"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
