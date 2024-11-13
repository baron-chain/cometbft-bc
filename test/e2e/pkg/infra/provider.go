package infra

import (
    "context"
    "time"
)

type Status string

const (
    StatusUnknown Status = "unknown"
    StatusReady Status = "ready"
    StatusError Status = "error"
    StatusInProgress Status = "in_progress"

    defaultTimeout = 30 * time.Second
    defaultRetryCount = 3
    
    // Baron Chain specific constants
    minCPUThreshold = 10.0
    maxCPUThreshold = 90.0
    minMemoryThreshold = 10.0
    maxMemoryThreshold = 90.0
    minDiskThreshold = 10.0
    maxDiskThreshold = 90.0
)

type Config struct {
    Timeout time.Duration
    RetryCount int
    Tags map[string]string
    
    // Baron Chain specific configurations
    QueueSize int
    MaxConnections int
    PersistInterval uint64
    SnapshotInterval uint64
}

type ResourceMetrics struct {
    CPUUsage float64
    MemoryUsage float64
    DiskUsage float64
    NetworkIn int64
    NetworkOut int64
    
    // Baron Chain specific metrics
    ActiveValidators int
    PendingTxs int
    BlockHeight int64
    ConsensusRound int
    PeerCount int
}

type Provider interface {
    Setup(ctx context.Context) error
    Teardown(ctx context.Context) error
    Status(ctx context.Context) (Status, error)
    GetMetrics(ctx context.Context) (*ResourceMetrics, error)
    IsHealthy(ctx context.Context) bool
    GetConfig() *Config
    UpdateConfig(cfg *Config) error
}

type BaseProvider struct {
    config *Config
    metrics *ResourceMetrics
    status Status
}

func NewBaseProvider(cfg *Config) *BaseProvider {
    if cfg == nil {
        cfg = &Config{
            Timeout: defaultTimeout,
            RetryCount: defaultRetryCount,
            Tags: make(map[string]string),
            QueueSize: 100,
            MaxConnections: 50,
            PersistInterval: 1,
            SnapshotInterval: 100,
        }
    }
    
    return &BaseProvider{
        config: cfg,
        metrics: &ResourceMetrics{},
        status: StatusUnknown,
    }
}

func (p *BaseProvider) GetConfig() *Config {
    return p.config
}

func (p *BaseProvider) UpdateConfig(cfg *Config) error {
    if err := validateConfig(cfg); err != nil {
        return err
    }
    p.config = cfg
    return nil
}

type NoopProvider struct {
    *BaseProvider
}

func NewNoopProvider(cfg *Config) *NoopProvider {
    return &NoopProvider{
        BaseProvider: NewBaseProvider(cfg),
    }
}

func (p *NoopProvider) Setup(ctx context.Context) error {
    return nil
}

func (p *NoopProvider) Teardown(ctx context.Context) error {
    return nil
}

func (p *NoopProvider) Status(ctx context.Context) (Status, error) {
    return StatusReady, nil
}

func (p *NoopProvider) GetMetrics(ctx context.Context) (*ResourceMetrics, error) {
    return &ResourceMetrics{}, nil
}

func (p *NoopProvider) IsHealthy(ctx context.Context) bool {
    return true
}

type Error struct {
    msg string
    code int
}

func NewError(msg string, code int) *Error {
    return &Error{
        msg: msg,
        code: code,
    }
}

func (e *Error) Error() string {
    return e.msg
}

func (e *Error) Code() int {
    return e.code
}

var (
    ErrInvalidConfig = NewError("invalid configuration", 1)
    ErrProvisionFailed = NewError("failed to provision resources", 2)
    ErrCleanupFailed = NewError("failed to cleanup resources", 3) 
    ErrMetricsUnavailable = NewError("metrics unavailable", 4)
)

// Helper functions

func validateConfig(cfg *Config) error {
    if cfg == nil {
        return ErrInvalidConfig
    }

    if cfg.Timeout < time.Second {
        return NewError("timeout must be at least 1 second", 5)
    }

    if cfg.RetryCount < 0 {
        return NewError("retry count cannot be negative", 6)
    }

    if cfg.QueueSize < 1 {
        return NewError("queue size must be positive", 7)
    }

    if cfg.MaxConnections < 1 {
        return NewError("max connections must be positive", 8)
    }

    return nil
}

func isMetricHealthy(value float64, min, max float64) bool {
    return value >= min && value <= max
}

func (p *BaseProvider) checkHealth() bool {
    if p.metrics == nil {
        return false
    }

    return isMetricHealthy(p.metrics.CPUUsage, minCPUThreshold, maxCPUThreshold) &&
           isMetricHealthy(p.metrics.MemoryUsage, minMemoryThreshold, maxMemoryThreshold) &&
           isMetricHealthy(p.metrics.DiskUsage, minDiskThreshold, maxDiskThreshold) &&
           p.metrics.PeerCount > 0 &&
           p.metrics.ActiveValidators > 0
}
