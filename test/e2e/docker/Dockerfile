# Build stage for Baron Chain
FROM golang:1.20 as builder

# Build arguments
ARG BARON_BUILD_OPTIONS="badgerdb,rocksdb"
ARG DEBIAN_FRONTEND=noninteractive
ARG GO111MODULE=on
ARG GOPROXY=direct

# Environment variables
ENV BARON_BUILD_OPTIONS=${BARON_BUILD_OPTIONS} \
    PATH="/usr/local/go/bin:$PATH" \
    CGO_ENABLED=1

# Install required packages
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends \
    libleveldb-dev \
    librocksdb-dev \
    build-essential \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set up build directory
WORKDIR /src/baron-chain

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build Baron Chain binaries
RUN make build && \
    cp build/baron-chain /usr/local/bin/ && \
    cd test/e2e && \
    make node && \
    cp build/node /usr/local/bin/baron-app

# Runtime stage
FROM debian:bullseye-slim

# Runtime arguments
ARG BARON_USER=baron
ARG BARON_GROUP=baron

# Install runtime dependencies
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends \
    libleveldb1d \
    librocksdb6.11 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create baron user and group
RUN groupadd -r ${BARON_GROUP} && \
    useradd -r -g ${BARON_GROUP} ${BARON_USER} && \
    mkdir -p /baron-chain /var/lib/baron && \
    chown -R ${BARON_USER}:${BARON_GROUP} /baron-chain /var/lib/baron

# Copy binaries and scripts from builder
COPY --from=builder /usr/local/bin/baron-chain /usr/local/bin/
COPY --from=builder /usr/local/bin/baron-app /usr/local/bin/
COPY --from=builder /src/baron-chain/test/e2e/docker/entrypoint* /usr/local/bin/

# Set runtime configuration
USER ${BARON_USER}
WORKDIR /baron-chain
VOLUME ["/baron-chain", "/var/lib/baron"]

# Environment variables
ENV BARON_HOME=/baron-chain \
    BARON_DATA=/var/lib/baron

# Expose ports:
# - 26656: P2P
# - 26657: RPC
# - 26660: Prometheus metrics
# - 6060: pprof
EXPOSE 26656 26657 26660 6060

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:26657/status || exit 1

# Set entrypoint and command
ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD ["node"]

# Use SIGTERM for graceful shutdown
STOPSIGNAL SIGTERM

# Labels
LABEL org.opencontainers.image.source="https://github.com/baron-chain/baron-chain" \
      org.opencontainers.image.description="Baron Chain Node" \
      org.opencontainers.image.licenses="Apache-2.0"
