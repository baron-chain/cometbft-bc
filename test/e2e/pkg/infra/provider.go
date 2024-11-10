// Package infra provides infrastructure management capabilities for testnets.
package infra

import (
	"context"
	"time"
)

// Status represents the current state of infrastructure resources.
type Status string

const (
	// StatusUnknown indicates the infrastructure state cannot be determined.
	StatusUnknown Status = "unknown"
	// StatusReady indicates the infrastructure is ready for use.
	StatusReady Status = "ready"
	// StatusError indicates the infrastructure is in an error state.
	StatusError Status = "error"
	// StatusInProgress indicates the infrastructure is being modified.
	StatusInProgress Status = "in_progress"
)

// Config holds provider configuration options.
type Config struct {
	// Timeout specifies the maximum duration for operations.
	Timeout time.Duration
	// RetryCount specifies the number of retry attempts for operations.
	RetryCount int
	// Tags contains provider-specific resource tags.
	Tags map[string]string
}

// ResourceMetrics contains infrastructure resource usage metrics.
type ResourceMetrics struct {
	// CPUUsage represents CPU utilization percentage.
	CPUUsage float64
	// MemoryUsage represents memory utilization percentage.
	MemoryUsage float64
	// DiskUsage represents disk utilization percentage.
	DiskUsage float64
	// NetworkIn represents incoming network traffic (bytes).
	NetworkIn int64
	// NetworkOut represents outgoing network traffic (bytes).
	NetworkOut int64
}

// Provider defines an API for managing testnet infrastructure.
type Provider interface {
	// Setup generates necessary configuration and provisions infrastructure resources.
	// Returns an error if setup fails.
	Setup(ctx context.Context) error

	// Teardown cleans up and releases infrastructure resources.
	// Returns an error if cleanup fails.
	Teardown(ctx context.Context) error

	// Status returns the current state of the infrastructure.
	Status(ctx context.Context) (Status, error)

	// GetMetrics returns current resource usage metrics.
	// Returns an error if metrics collection fails.
	GetMetrics(ctx context.Context) (*ResourceMetrics, error)

	// IsHealthy performs a health check of the infrastructure.
	// Returns true if healthy, false otherwise.
	IsHealthy(ctx context.Context) bool

	// GetConfig returns the current provider configuration.
	GetConfig() *Config

	// UpdateConfig updates the provider configuration.
	// Returns an error if update fails.
	UpdateConfig(cfg *Config) error
}

// BaseProvider implements common functionality for infrastructure providers.
type BaseProvider struct {
	config *Config
}

// NewBaseProvider creates a new BaseProvider with the given configuration.
func NewBaseProvider(cfg *Config) *BaseProvider {
	if cfg == nil {
		cfg = &Config{
			Timeout:    30 * time.Second,
			RetryCount: 3,
			Tags:      make(map[string]string),
		}
	}
	return &BaseProvider{config: cfg}
}

// GetConfig returns the current provider configuration.
func (p *BaseProvider) GetConfig() *Config {
	return p.config
}

// UpdateConfig updates the provider configuration.
func (p *BaseProvider) UpdateConfig(cfg *Config) error {
	if cfg == nil {
		return ErrInvalidConfig
	}
	p.config = cfg
	return nil
}

// NoopProvider implements the Provider interface by performing no-ops.
// This is useful when infrastructure is managed externally.
type NoopProvider struct {
	*BaseProvider
}

// NewNoopProvider creates a new NoopProvider instance.
func NewNoopProvider(cfg *Config) *NoopProvider {
	return &NoopProvider{
		BaseProvider: NewBaseProvider(cfg),
	}
}

// Setup implements Provider.Setup as a no-op.
func (p *NoopProvider) Setup(ctx context.Context) error {
	return nil
}

// Teardown implements Provider.Teardown as a no-op.
func (p *NoopProvider) Teardown(ctx context.Context) error {
	return nil
}

// Status implements Provider.Status as a no-op.
func (p *NoopProvider) Status(ctx context.Context) (Status, error) {
	return StatusReady, nil
}

// GetMetrics implements Provider.GetMetrics as a no-op.
func (p *NoopProvider) GetMetrics(ctx context.Context) (*ResourceMetrics, error) {
	return &ResourceMetrics{}, nil
}

// IsHealthy implements Provider.IsHealthy as a no-op.
func (p *NoopProvider) IsHealthy(ctx context.Context) bool {
	return true
}

// Common errors returned by providers.
var (
	ErrInvalidConfig      = New("invalid configuration")
	ErrProvisionFailed    = New("failed to provision resources")
	ErrCleanupFailed      = New("failed to cleanup resources")
	ErrMetricsUnavailable = New("metrics unavailable")
)

// Error represents an infrastructure provider error.
type Error struct {
	msg string
}

// New creates a new Error instance.
func New(msg string) error {
	return &Error{msg: msg}
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.msg
}
