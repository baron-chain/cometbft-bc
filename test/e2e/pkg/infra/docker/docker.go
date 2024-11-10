// Package docker provides a Docker Compose-based infrastructure provider for testnets.
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

	e2e "github.com/cometbft/cometbft/test/e2e/pkg"
	"github.com/cometbft/cometbft/test/e2e/pkg/infra"
)

const (
	// Default file permissions for generated files
	defaultFilePerms = 0644
	
	// Composition file name
	composeFileName = "docker-compose.yml"
	
	// Default network subnet
	defaultSubnet = "172.57.0.0/16"
	
	// Default timeout for Docker operations
	defaultTimeout = 30 * time.Second
)

// DockerConfig contains Docker-specific configuration options
type DockerConfig struct {
	// ComposeVersion specifies the Docker Compose file version
	ComposeVersion string
	// NetworkDriver specifies the Docker network driver
	NetworkDriver string
	// IPAMDriver specifies the IPAM driver
	IPAMDriver string
	// Subnet specifies the network subnet
	Subnet string
	// Labels contains Docker resource labels
	Labels map[string]string
}

// Provider implements a Docker Compose-backed infrastructure provider.
type Provider struct {
	*infra.BaseProvider
	Testnet *e2e.Testnet
	Config  *DockerConfig
}

// NewProvider creates a new Docker provider instance.
func NewProvider(testnet *e2e.Testnet, cfg *DockerConfig) *Provider {
	if cfg == nil {
		cfg = defaultConfig()
	}
	return &Provider{
		BaseProvider: infra.NewBaseProvider(&infra.Config{
			Timeout:    defaultTimeout,
			RetryCount: 3,
			Tags:       cfg.Labels,
		}),
		Testnet: testnet,
		Config:  cfg,
	}
}

// defaultConfig returns the default Docker configuration.
func defaultConfig() *DockerConfig {
	return &DockerConfig{
		ComposeVersion: "2.4",
		NetworkDriver:  "bridge",
		IPAMDriver:    "default",
		Subnet:        defaultSubnet,
		Labels: map[string]string{
			"e2e": "true",
		},
	}
}

// Setup implements infra.Provider.
func (p *Provider) Setup(ctx context.Context) error {
	// Generate Docker Compose config
	compose, err := p.generateComposeConfig()
	if err != nil {
		return fmt.Errorf("failed to generate compose config: %w", err)
	}

	// Write config to file
	composePath := filepath.Join(p.Testnet.Dir, composeFileName)
	if err := os.WriteFile(composePath, compose, defaultFilePerms); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Start the containers
	return p.startContainers(ctx)
}

// Teardown implements infra.Provider.
func (p *Provider) Teardown(ctx context.Context) error {
	return p.stopContainers(ctx)
}

// Status implements infra.Provider.
func (p *Provider) Status(ctx context.Context) (infra.Status, error) {
	if err := p.checkContainers(ctx); err != nil {
		return infra.StatusError, err
	}
	return infra.StatusReady, nil
}

// GetMetrics implements infra.Provider.
func (p *Provider) GetMetrics(ctx context.Context) (*infra.ResourceMetrics, error) {
	stats, err := p.collectContainerStats(ctx)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// IsHealthy implements infra.Provider.
func (p *Provider) IsHealthy(ctx context.Context) bool {
	status, _ := p.Status(ctx)
	return status == infra.StatusReady
}

// generateComposeConfig generates the Docker Compose configuration.
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
		Config:  p.Config,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// startContainers starts the Docker containers.
func (p *Provider) startContainers(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker-compose", "-f", 
		filepath.Join(p.Testnet.Dir, composeFileName), "up", "-d")
	return cmd.Run()
}

// stopContainers stops and removes the Docker containers.
func (p *Provider) stopContainers(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker-compose", "-f",
		filepath.Join(p.Testnet.Dir, composeFileName), "down", "--volumes")
	return cmd.Run()
}

// checkContainers verifies all containers are running.
func (p *Provider) checkContainers(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker-compose", "-f",
		filepath.Join(p.Testnet.Dir, composeFileName), "ps", "-q")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if len(output) == 0 {
		return fmt.Errorf("no containers running")
	}
	return nil
}

// collectContainerStats collects resource usage metrics from containers.
func (p *Provider) collectContainerStats(ctx context.Context) (*infra.ResourceMetrics, error) {
	// Implementation for collecting Docker stats
	// This would use Docker API to get container statistics
	return &infra.ResourceMetrics{}, nil
}

// Docker Compose template
const composeTemplate = ` + "`" + `version: '{{ .Config.ComposeVersion }}'
networks:
  {{ .Name }}:
    labels:
      {{- range $key, $value := .Config.Labels }}
      {{ $key }}: {{ $value }}
      {{- end }}
    driver: {{ .Config.NetworkDriver }}
{{- if .IPv6 }}
    enable_ipv6: true
{{- end }}
    ipam:
      driver: {{ .Config.IPAMDriver }}
      config:
      - subnet: {{ or .IP .Config.Subnet }}

services:
{{- range .Nodes }}
  {{ .Name }}:
    labels:
      {{- range $key, $value := $.Config.Labels }}
      {{ $key }}: {{ $value }}
      {{- end }}
    container_name: {{ .Name }}
    image: {{ .Version }}
{{- if or (eq .ABCIProtocol "builtin") (eq .ABCIProtocol "builtin_unsync") }}
    entrypoint: /usr/bin/entrypoint-builtin
{{- end }}
    init: true
    ports:
    - 26656
    - {{ if .ProxyPort }}{{ .ProxyPort }}:{{ end }}26657
{{- if .PrometheusProxyPort }}
    - {{ .PrometheusProxyPort }}:26660
{{- end }}
    - 6060
    volumes:
    - ./{{ .Name }}:/cometbft
    - ./{{ .Name }}:/tendermint
    networks:
      {{ $.Name }}:
        ipv{{ if $.IPv6 }}6{{ else }}4{{ end}}_address: {{ .IP }}

{{- if ne .Version $.UpgradeVersion}}
  {{ .Name }}_u:
    labels:
      {{- range $key, $value := $.Config.Labels }}
      {{ $key }}: {{ $value }}
      {{- end }}
    container_name: {{ .Name }}_u
    image: {{ $.UpgradeVersion }}
{{- if or (eq .ABCIProtocol "builtin") (eq .ABCIProtocol "builtin_unsync") }}
    entrypoint: /usr/bin/entrypoint-builtin
{{- end }}
    init: true
    ports:
    - 26656
    - {{ if .ProxyPort }}{{ .ProxyPort }}:{{ end }}26657
{{- if .PrometheusProxyPort }}
    - {{ .PrometheusProxyPort }}:26660
{{- end }}
    - 6060
    volumes:
    - ./{{ .Name }}:/cometbft
    - ./{{ .Name }}:/tendermint
    networks:
      {{ $.Name }}:
        ipv{{ if $.IPv6 }}6{{ else }}4{{ end}}_address: {{ .IP }}
{{- end }}
{{- end }}` + "`" + `

// Ensure Provider implements the interface
var _ infra.Provider = &Provider{}
