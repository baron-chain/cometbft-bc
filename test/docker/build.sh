#!/bin/bash

# Exit on error
set -e

# Define variables
DOCKER_IMAGE="baron-chain-tester"
DOCKER_TAG="latest"
DOCKERFILE_PATH="./test/docker/Dockerfile"
BUILD_ARGS=""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker first."
    exit 1
fi

# Add build arguments for PQC and AI components
BUILD_ARGS="$BUILD_ARGS --build-arg KYBER_VERSION=v1.0.0"
BUILD_ARGS="$BUILD_ARGS --build-arg DILITHIUM_VERSION=v1.0.0"
BUILD_ARGS="$BUILD_arg AI_MODEL_VERSION=v1.0.0"

# Build the Docker image with optimizations
docker build \
    --no-cache \
    --compress \
    --force-rm \
    --pull \
    $BUILD_ARGS \
    -t $DOCKER_IMAGE:$DOCKER_TAG \
    -f $DOCKERFILE_PATH \
    --target release \
    .

# Verify the build
if [ $? -eq 0 ]; then
    echo "✅ Baron Chain Docker image built successfully"
    docker images | grep $DOCKER_IMAGE
else
    echo "❌ Failed to build Baron Chain Docker image"
    exit 1
fi
