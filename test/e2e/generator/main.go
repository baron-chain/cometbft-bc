package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/baron-chain/cometbft-bc/crypto/rand"
	"github.com/baron-chain/cometbft-bc/libs/log"
	bcconfig "github.com/baron-chain/cometbft-bc/config"
	"github.com/spf13/cobra"
)

const (
	defaultSeed = 4827085738
	appName     = "baron-generator"
)

var (
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	config = bcconfig.DefaultConfig()
)

type Generator struct {
	cmd    *cobra.Command
	config *GeneratorConfig
}

type GeneratorConfig struct {
	outputDir     string
	groups        int
	versions      string
	metrics       bool
	rand          *rand.Rand
}

func NewGenerator() *Generator {
	g := &Generator{
		config: &GeneratorConfig{
			rand: rand.NewRand(),
		},
	}
	g.setupCommands()
	return g
}

func (g *Generator) setupCommands() {
	g.cmd = &cobra.Command{
		Use:   fmt.Sprintf("%s --dir <output-dir> [flags]", appName),
		Short: "Baron Chain testnet generator",
		Long: `Generates Baron Chain testnet configurations for E2E testing.
Supports multi-version testing and Prometheus metrics integration.`,
		RunE: g.runGenerate,
	}

	flags := g.cmd.PersistentFlags()
	flags.StringVarP(&g.config.outputDir, "dir", "d", "", "Output directory for testnet configs")
	flags.IntVarP(&g.config.groups, "groups", "g", 0, "Number of testnet groups")
	flags.StringVarP(&g.config.versions, "versions", "v", "", 
		"Comma-separated Baron Chain versions to test (empty = current version)")
	flags.BoolVarP(&g.config.metrics, "metrics", "m", false, "Enable Prometheus metrics")

	g.cmd.MarkPersistentFlagRequired("dir")
}

func (g *Generator) runGenerate(cmd *cobra.Command, args []string) error {
	if err := g.validateConfig(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	logger.Info("generating testnet configurations", 
		"dir", g.config.outputDir,
		"groups", g.config.groups,
		"metrics", g.config.metrics)

	manifests, err := g.generateManifests()
	if err != nil {
		return fmt.Errorf("manifest generation failed: %w", err)
	}

	return g.saveManifests(manifests)
}

func (g *Generator) validateConfig() error {
	if err := os.MkdirAll(g.config.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if g.config.groups < 0 {
		return fmt.Errorf("invalid group count: %d", g.config.groups)
	}

	return nil
}

func (g *Generator) generateManifests() ([]Manifest, error) {
	cfg := &ManifestConfig{
		Versions:    g.config.versions,
		EnableMetrics: g.config.metrics,
		ChainID:     fmt.Sprintf("baron-test-%d", g.config.rand.Int63()),
	}

	manifests, err := GenerateManifests(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate manifests: %w", err)
	}

	return manifests, nil
}

func (g *Generator) saveManifests(manifests []Manifest) error {
	if g.config.groups <= 0 {
		return g.saveFlatManifests(manifests)
	}
	return g.saveGroupedManifests(manifests)
}

func (g *Generator) saveFlatManifests(manifests []Manifest) error {
	for i, m := range manifests {
		filename := filepath.Join(g.config.outputDir, fmt.Sprintf("baron-net-%04d.toml", i))
		if err := m.Save(filename); err != nil {
			return fmt.Errorf("failed to save manifest %d: %w", i, err)
		}
		logger.Info("saved manifest", "file", filename)
	}
	return nil
}

func (g *Generator) saveGroupedManifests(manifests []Manifest) error {
	groupSize := int(math.Ceil(float64(len(manifests)) / float64(g.config.groups)))

	for group := 0; group < g.config.groups; group++ {
		for i := 0; i < groupSize && group*groupSize+i < len(manifests); i++ {
			manifest := manifests[group*groupSize+i]
			filename := filepath.Join(g.config.outputDir, 
				fmt.Sprintf("baron-net-g%02d-%04d.toml", group, i))
			
			if err := manifest.Save(filename); err != nil {
				return fmt.Errorf("failed to save manifest group %d-%d: %w", group, i, err)
			}
			logger.Info("saved manifest", "group", group, "file", filename)
		}
	}
	return nil
}

func main() {
	generator := NewGenerator()
	if err := generator.cmd.Execute(); err != nil {
		logger.Error("generator failed", "err", err)
		os.Exit(1)
	}
}
