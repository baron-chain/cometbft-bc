package docker

import (
    "bytes"
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "text/template"
    "time"

    e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
    "github.com/baron-chain/cometbft-bc/test/e2e/pkg/infra"
)

const (
    defaultFilePerms = 0644
    composeFileName = "docker-compose.yml"
    defaultSubnet = "172.57.0.0/16"
    defaultTimeout = 30 * time.Second
    
    // Baron Chain specific constants
    baronNetworkName = "baron-network"
    defaultNodeImage = "baron-chain/node:latest"
    metricsPort = 26660
    p2pPort = 26656
    rpcPort = 26657
)

type DockerConfig struct {
    ComposeVersion string
    NetworkDriver string
    IPAMDriver string
    Subnet string
    Labels map[string]string
    
    // Baron Chain specific configs
    NodeImage string
    EnableMetrics bool
    EnablePersistence bool
    PersistPath string
    ValidatorSetSize int
}

type Provider struct {
    *infra.BaseProvider
    Testnet *e2e.Testnet
    Config *DockerConfig
}

func NewProvider(testnet *e2e.Testnet, cfg *DockerConfig) *Provider {
    if cfg == nil {
        cfg = defaultConfig()
    }
    return &Provider{
        BaseProvider: infra.NewBaseProvider(&infra.Config{
            Timeout: defaultTimeout,
            RetryCount: 3,
            Tags: cfg.Labels,
        }),
        Testnet: testnet,
        Config: cfg,
    }
}

func defaultConfig() *DockerConfig {
    return &DockerConfig{
        ComposeVersion: "3.8",
        NetworkDriver: "bridge",
        IPAMDriver: "default",
        Subnet: defaultSubnet,
        NodeImage: defaultNodeImage,
        EnableMetrics: true,
        EnablePersistence: true,
        PersistPath: "/data",
        ValidatorSetSize: 4,
        Labels: map[string]string{
            "network": baronNetworkName,
            "chain": "baron",
        },
    }
}

func (p *Provider) Setup(ctx context.Context) error {
    compose, err := p.generateComposeConfig()
    if err != nil {
        return fmt.Errorf("failed to generate docker compose config: %w", err)
    }

    composePath := filepath.Join(p.Testnet.Dir, composeFileName)
    if err := os.WriteFile(composePath, compose, defaultFilePerms); err != nil {
        return fmt.Errorf("failed to write compose file: %w", err)
    }

    if err := p.startContainers(ctx); err != nil {
        return fmt.Errorf("failed to start containers: %w", err)
    }

    return p.waitForHealthy(ctx)
}

func (p *Provider) Teardown(ctx context.Context) error {
    if err := p.stopContainers(ctx); err != nil {
        return fmt.Errorf("failed to stop containers: %w", err)
    }
    return p.cleanup(ctx)
}

func (p *Provider) Status(ctx context.Context) (infra.Status, error) {
    healthy, err := p.checkHealth(ctx)
    if err != nil {
        return infra.StatusError, err
    }
    if !healthy {
        return infra.StatusError, fmt.Errorf("network unhealthy")
    }
    return infra.StatusReady, nil
}

func (p *Provider) GetMetrics(ctx context.Context) (*infra.ResourceMetrics, error) {
    return p.collectMetrics(ctx)
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
    healthy, _ := p.checkHealth(ctx)
    return healthy
}

// Helper functions

func (p *Provider) generateComposeConfig() ([]byte, error) {
    tmpl, err := template.New("docker-compose").Parse(composeTemplate)
    if err != nil {
        return nil, err
    }

    data := struct {
        *e2e.Testnet
        Config *DockerConfig
    }{
        Testnet: p.Testnet,
        Config: p.Config,
    }

    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

func (p *Provider) startContainers(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "docker-compose", "-f",
        filepath.Join(p.Testnet.Dir, composeFileName), "up", "-d")
    cmd.Env = append(os.Environ(), "COMPOSE_PROJECT_NAME="+baronNetworkName)
    return cmd.Run()
}

func (p *Provider) stopContainers(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "docker-compose", "-f",
        filepath.Join(p.Testnet.Dir, composeFileName), "down", "--volumes", "--remove-orphans")
    cmd.Env = append(os.Environ(), "COMPOSE_PROJECT_NAME="+baronNetworkName)
    return cmd.Run()
}

func (p *Provider) checkHealth(ctx context.Context) (bool, error) {
    cmd := exec.CommandContext(ctx, "docker-compose", "-f",
        filepath.Join(p.Testnet.Dir, composeFileName), "ps", "--format", "json")
    
    output, err := cmd.Output()
    if err != nil {
        return false, err
    }

    return len(output) > 0, nil
}

func (p *Provider) collectMetrics(ctx context.Context) (*infra.ResourceMetrics, error) {
    metrics := &infra.ResourceMetrics{}
    
    // Collect container stats using Docker API
    for _, node := range p.Testnet.Nodes {
        stats, err := p.getContainerStats(ctx, node.Name)
        if err != nil {
            continue
        }
        metrics.CPUUsage += stats.CPUUsage
        metrics.MemoryUsage += stats.MemoryUsage
    }

    return metrics, nil
}

func (p *Provider) waitForHealthy(ctx context.Context) error {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if healthy, _ := p.checkHealth(ctx); healthy {
                return nil
            }
        }
    }
}

func (p *Provider) cleanup(ctx context.Context) error {
    return os.RemoveAll(p.Testnet.Dir)
}

// Docker compose template with improved formatting and Baron Chain specifics
const composeTemplate = `version: '{{ .Config.ComposeVersion }}'

networks:
  {{ .Name }}:
    name: {{ .Name }}
    driver: {{ .Config.NetworkDriver }}
    ipam:
      driver: {{ .Config.IPAMDriver }}
      config:
        - subnet: {{ .Config.Subnet }}
    labels:
      {{- range $key, $value := .Config.Labels }}
      {{ $key }}: {{ $value }}
      {{- end }}

services:
{{- range .Nodes }}
  {{ .Name }}:
    container_name: {{ .Name }}
    image: {{ $.Config.NodeImage }}
    networks:
      {{ $.Name }}:
        ipv4_address: {{ .IP }}
    volumes:
      - {{ $.Config.PersistPath }}/{{ .Name }}:/data
    ports:
      - "{{ .ProxyPort }}:{{ rpcPort }}"
      {{- if $.Config.EnableMetrics }}
      - "{{ .PrometheusProxyPort }}:{{ metricsPort }}"
      {{- end }}
    environment:
      - BARON_HOME=/data
      - BARON_CHAIN_ID={{ $.Name }}
      {{- if eq .Mode "validator" }}
      - BARON_VALIDATOR=true
      {{- end }}
    command: start
    healthcheck:
      test: ["CMD", "baron", "status"]
      interval: 5s
      timeout: 5s
      retries: 5
{{- end }}
`

var _ infra.Provider = &Provider{}
