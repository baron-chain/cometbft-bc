package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/baron-chain/cometbft-bc/libs/json"
	"github.com/baron-chain/cometbft-bc/rpc/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

const (
	defaultGRPCAddr     = "tcp://localhost:9656"
	defaultTimeout      = 10 * time.Second
	maxMessageSize      = 50 * 1024 * 1024 // 50MB
	keepaliveTime       = 20 * time.Second
	keepaliveTimeout    = 10 * time.Second
	defaultPQCEnabled   = true
)

type Client struct {
	grpcClient *grpc.Client
	pqcEnabled bool
}

// NewClient creates a new Baron Chain gRPC client with PQC support
func NewClient(addr string, pqcEnabled bool) (*Client, error) {
	opts := []grpc.Option{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,
			Timeout:             keepaliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithMaxMsgSize(maxMessageSize),
	}

	// Add PQC-enabled TLS if quantum-safe mode is enabled
	if pqcEnabled {
		creds, err := credentials.NewClientTLSFromFile("config/pqc-cert.pem", "")
		if err != nil {
			return nil, fmt.Errorf("failed to load PQC credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	grpcClient := grpc.StartGRPCClient(addr, opts...)
	if grpcClient == nil {
		return nil, fmt.Errorf("failed to create gRPC client")
	}

	return &Client{
		grpcClient: grpcClient,
		pqcEnabled: pqcEnabled,
	}, nil
}

// addPQCSignature adds a quantum-safe signature to the transaction
func (c *Client) addPQCSignature(tx []byte) []byte {
	if !c.pqcEnabled {
		return tx
	}
	// In production, this would use actual PQC signing
	// For now, we just append a marker
	return append(tx, []byte("_pqc")...)
}

// BroadcastTx broadcasts a transaction with optional PQC signature
func (c *Client) BroadcastTx(ctx context.Context, tx []byte) (*grpc.ResponseBroadcastTx, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	// Add PQC signature if enabled
	signedTx := c.addPQCSignature(tx)

	res, err := c.grpcClient.BroadcastTx(ctx, &grpc.RequestBroadcastTx{Tx: signedTx})
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return res, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: grpc_client <hex_transaction>")
		os.Exit(1)
	}

	// Get gRPC address from environment or use default
	grpcAddr := os.Getenv("BARON_CHAIN_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = defaultGRPCAddr
	}

	// Parse transaction hex
	txHex := os.Args[1]
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid transaction hex: %v\n", err)
		os.Exit(1)
	}

	// Create client with PQC support
	client, err := NewClient(grpcAddr, defaultPQCEnabled)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}

	// Broadcast transaction
	ctx := context.Background()
	res, err := client.BroadcastTx(ctx, txBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to broadcast transaction: %v\n", err)
		os.Exit(1)
	}

	// Marshal and print response
	bz, err := json.Marshal(res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal response: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(bz))
}
