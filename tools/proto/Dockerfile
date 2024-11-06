# syntax=docker/dockerfile:1.4

# Build stage for Protocol Buffer tooling
FROM bufbuild/buf:latest AS buf

# Builder stage for Go tools
FROM golang:1.21-alpine3.19 AS builder

# Build arguments for version control
ARG GOLANG_PROTOBUF_VERSION=1.3.1
ARG GOGOPROTO_VERSION=1.4.1
ARG ALPINE_VERSION=3.19

# Metadata labels
LABEL maintainer="Your Name <your.email@example.com>"
LABEL description="Protocol Buffer and Go development toolchain"
LABEL version="1.0"

# Install build dependencies
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
        build-base \
        curl \
        git \
        upx

# Install Go protobuf tools
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install \
        github.com/golang/protobuf/protoc-gen-go@v${GOLANG_PROTOBUF_VERSION} \
        github.com/cosmos/gogoproto/protoc-gen-gogo@v${GOGOPROTO_VERSION} \
        github.com/cosmos/gogoproto/protoc-gen-gogofaster@v${GOGOPROTO_VERSION} && \
    mv /go/bin/protoc-gen-go* /usr/local/bin/

# Final stage
FROM alpine:${ALPINE_VERSION}

# Create non-root user
RUN adduser -D -h /work developer

# Set working directory
WORKDIR /work

# Install clang from testing repository
RUN --mount=type=cache,target=/var/cache/apk \
    echo 'http://dl-cdn.alpinelinux.org/alpine/edge/testing' >> /etc/apk/repositories && \
    apk add --no-cache clang

# Copy binaries from previous stages
COPY --from=builder /usr/local/bin/ /usr/local/bin/
COPY --from=buf /usr/local/bin/ /usr/local/bin/

# Set user for better security
USER developer

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
    CMD clang --version || exit 1

# Default command
CMD ["clang", "--version"]
