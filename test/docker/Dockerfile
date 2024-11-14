# Build stage
FROM golang:1.21-alpine AS builder

# Build arguments for versioning
ARG KYBER_VERSION=v1.0.0
ARG DILITHIUM_VERSION=v1.0.0
ARG AI_MODEL_VERSION=v1.0.0

# Install build dependencies
RUN apk add --no-cache \
    build-base \
    git \
    linux-headers \
    bash \
    ca-certificates \
    curl \
    make

# Setup Baron Chain repo
ENV REPO=/baron-chain
ENV GOBIN=/go/bin
WORKDIR $REPO

# Copy only necessary files first
COPY go.mod go.sum Makefile ./
COPY scripts ./scripts

# Download dependencies
RUN go mod download

# Install tools and ABCI
RUN make tools && make install_abci

# Copy the rest of the source code
COPY . .

# Build Baron Chain with PQC support
RUN CGO_ENABLED=1 make install

# Generate testnet configuration
RUN baron-chain testnet \
    --config $REPO/test/docker/config-template.toml \
    --node-dir-prefix="node" \
    --v=4 \
    --populate-persistent-peers=false \
    --o=$REPO/test/p2p/data

# Production stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    bash \
    jq \
    curl \
    netcat-openbsd \
    libgcc \
    libstdc++ \
    vim

# Copy necessary binaries and configs from builder
COPY --from=builder /go/bin/baron-chain /usr/local/bin/
COPY --from=builder /baron-chain/test/p2p/data /baron-chain/data

# Setup quantum-safe components
RUN mkdir -p /etc/baron-chain/pqc && \
    chmod 700 /etc/baron-chain/pqc

# Setup AI model directory
RUN mkdir -p /etc/baron-chain/ai-models && \
    chmod 755 /etc/baron-chain/ai-models

# Create baron-chain user
RUN adduser -D -h /baron-chain baron-chain && \
    chown -R baron-chain:baron-chain /baron-chain /etc/baron-chain

# Switch to non-root user
USER baron-chain
WORKDIR /baron-chain

# Expose ports
EXPOSE 9656 9657

# Set environment variables
ENV BARON_CHAIN_HOME=/baron-chain
ENV PQC_ENABLED=true
ENV AI_OPTIMIZATION=true

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:9657/status || exit 1

# Default command
CMD ["baron-chain", "start", "--log_level", "info"]
