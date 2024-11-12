package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/baron-chain/cometbft-bc/libs/log"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
	"github.com/baron-chain/cometbft-bc/test/e2e/pkg/infra"
	"github.com/baron-chain/cometbft-bc/test/e2e/pkg/infra/docker"
)

const (
	randomSeed = 2308084734268
)

var (
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Common errors
	ErrInvalidTestnet = errors.New("invalid testnet configuration")
	ErrMissingConfig  = errors.New("missing testnet configuration")
)

// CLI represents the command-line interface application
type CLI struct {
	root     *cobra.Command
	testnet  *e2e.Testnet
	preserve bool
	infp     infra.Provider
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	cli := &CLI{}
	cli.root = cli.setupRootCommand()
	return cli
}

// Run executes the CLI application
func (cli *CLI) Run() {
	if err := cli.root.Execute(); err != nil {
		logger.Error("cli execution failed", "error", err)
		os.Exit(1)
	}
}

// setupRootCommand configures the root command and its flags
func (cli *CLI) setupRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e-test",
		Short: "End-to-end test runner",
		Long:  "Runs end-to-end tests for CometBFT networks",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cli.handlePreRun(cmd, args)
		},
	}

	cli.addGlobalFlags(cmd)
	cli.addCommands(cmd)

	return cmd
}

// addGlobalFlags adds global flags to the root command
func (cli *CLI) addGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("testnet", "t", "", "Path to testnet manifest file")
	cmd.PersistentFlags().BoolP("preserve", "p", false, "Preserve testnet directory after running tests")
}

// addCommands adds subcommands to the root command
func (cli *CLI) addCommands(cmd *cobra.Command) {
	// Add your subcommands here
	// Example:
	// cmd.AddCommand(cli.newStartCommand())
	// cmd.AddCommand(cli.newStopCommand())
}

// handlePreRun handles common setup before running commands
func (cli *CLI) handlePreRun(cmd *cobra.Command, args []string) error {
	testnetFile, err := cmd.Flags().GetString("testnet")
	if err != nil {
		return fmt.Errorf("failed to get testnet flag: %w", err)
	}

	if testnetFile == "" {
		return ErrMissingConfig
	}

	preserve, err := cmd.Flags().GetBool("preserve")
	if err != nil {
		return fmt.Errorf("failed to get preserve flag: %w", err)
	}
	cli.preserve = preserve

	testnet, err := LoadTestnet(testnetFile)
	if err != nil {
		return fmt.Errorf("failed to load testnet: %w", err)
	}
	cli.testnet = testnet

	if err := cli.setupInfraProvider(); err != nil {
		return fmt.Errorf("failed to setup infrastructure provider: %w", err)
	}

	return nil
}

// LoadTestnet loads a testnet configuration from a file
func LoadTestnet(file string) (*e2e.Testnet, error) {
	testnet, err := e2e.LoadTestnet(file)
	if err != nil {
		return nil, fmt.Errorf("failed to load testnet %q: %w", file, err)
	}

	if err := validateTestnet(testnet); err != nil {
		return nil, err
	}

	return testnet, nil
}

// validateTestnet validates the testnet configuration
func validateTestnet(testnet *e2e.Testnet) error {
	if testnet == nil {
		return ErrInvalidTestnet
	}

	if len(testnet.Nodes) == 0 {
		return fmt.Errorf("%w: no nodes specified", ErrInvalidTestnet)
	}

	return nil
}

// setupInfraProvider initializes the infrastructure provider
func (cli *CLI) setupInfraProvider() error {
	var err error
	cli.infp, err = docker.NewProvider(cli.testnet)
	if err != nil {
		return fmt.Errorf("failed to create docker provider: %w", err)
	}
	return nil
}

// cleanup performs cleanup operations
func (cli *CLI) cleanup(ctx context.Context) error {
	if cli.preserve {
		logger.Info("Preserving testnet directory", "dir", cli.testnet.Dir)
		return nil
	}

	if err := cli.infp.Clean(ctx); err != nil {
		return fmt.Errorf("failed to clean up infrastructure: %w", err)
	}

	return os.RemoveAll(cli.testnet.Dir)
}

func main() {
	// Set random seed for reproducibility
	rand.Seed(randomSeed)
	
	NewCLI().Run()
}
// NewCLI sets up the CLI.
func NewCLI() *CLI {
	cli := &CLI{}
	cli.root = &cobra.Command{
		Use:           "runner",
		Short:         "End-to-end test runner",
		SilenceUsage:  true,
		SilenceErrors: true, // we'll output them ourselves in Run()
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			file, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}
			m, err := e2e.LoadManifest(file)
			if err != nil {
				return err
			}

			inft, err := cmd.Flags().GetString("infrastructure-type")
			if err != nil {
				return err
			}

			var ifd e2e.InfrastructureData
			switch inft {
			case "docker":
				var err error
				ifd, err = e2e.NewDockerInfrastructureData(m)
				if err != nil {
					return err
				}
			case "digital-ocean":
				p, err := cmd.Flags().GetString("infrastructure-data")
				if err != nil {
					return err
				}
				if p == "" {
					return errors.New("'--infrastructure-data' must be set when using the 'digital-ocean' infrastructure-type")
				}
				ifd, err = e2e.InfrastructureDataFromFile(p)
				if err != nil {
					return fmt.Errorf("parsing infrastructure data: %s", err)
				}
			default:
				return fmt.Errorf("unknown infrastructure type '%s'", inft)
			}

			testnet, err := e2e.LoadTestnet(m, file, ifd)
			if err != nil {
				return fmt.Errorf("loading testnet: %s", err)
			}

			cli.testnet = testnet
			cli.infp = &infra.NoopProvider{}
			if inft == "docker" {
				cli.infp = &docker.Provider{Testnet: testnet}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := Cleanup(cli.testnet); err != nil {
				return err
			}
			if err := Setup(cli.testnet, cli.infp); err != nil {
				return err
			}

			r := rand.New(rand.NewSource(randomSeed)) //nolint: gosec

			chLoadResult := make(chan error)
			ctx, loadCancel := context.WithCancel(context.Background())
			defer loadCancel()
			go func() {
				err := Load(ctx, cli.testnet)
				if err != nil {
					logger.Error(fmt.Sprintf("Transaction load failed: %v", err.Error()))
				}
				chLoadResult <- err
			}()

			if err := Start(cli.testnet); err != nil {
				return err
			}

			if err := Wait(cli.testnet, 5); err != nil { // allow some txs to go through
				return err
			}

			if cli.testnet.HasPerturbations() {
				if err := Perturb(cli.testnet); err != nil {
					return err
				}
				if err := Wait(cli.testnet, 5); err != nil { // allow some txs to go through
					return err
				}
			}

			if cli.testnet.Evidence > 0 {
				if err := InjectEvidence(ctx, r, cli.testnet, cli.testnet.Evidence); err != nil {
					return err
				}
				if err := Wait(cli.testnet, 5); err != nil { // ensure chain progress
					return err
				}
			}

			loadCancel()
			if err := <-chLoadResult; err != nil {
				return err
			}
			if err := Wait(cli.testnet, 5); err != nil { // wait for network to settle before tests
				return err
			}
			if err := Test(cli.testnet); err != nil {
				return err
			}
			if !cli.preserve {
				if err := Cleanup(cli.testnet); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cli.root.PersistentFlags().StringP("file", "f", "", "Testnet TOML manifest")
	_ = cli.root.MarkPersistentFlagRequired("file")

	cli.root.PersistentFlags().StringP("infrastructure-type", "", "docker", "Backing infrastructure used to run the testnet. Either 'digital-ocean' or 'docker'")

	cli.root.PersistentFlags().StringP("infrastructure-data", "", "", "path to the json file containing the infrastructure data. Only used if the 'infrastructure-type' is set to a value other than 'docker'")

	cli.root.Flags().BoolVarP(&cli.preserve, "preserve", "p", false,
		"Preserves the running of the test net after tests are completed")

	cli.root.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Generates the testnet directory and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Setup(cli.testnet, cli.infp)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Starts the Docker testnet, waiting for nodes to become available",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stat(cli.testnet.Dir)
			if os.IsNotExist(err) {
				err = Setup(cli.testnet, cli.infp)
			}
			if err != nil {
				return err
			}
			return Start(cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "perturb",
		Short: "Perturbs the Docker testnet, e.g. by restarting or disconnecting nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Perturb(cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "wait",
		Short: "Waits for a few blocks to be produced and all nodes to catch up",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Wait(cli.testnet, 5)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stops the Docker testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Stopping testnet")
			return execCompose(cli.testnet.Dir, "down")
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "load",
		Short: "Generates transaction load until the command is canceled",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return Load(context.Background(), cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "evidence [amount]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generates and broadcasts evidence to a random node",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			amount := 1

			if len(args) == 1 {
				amount, err = strconv.Atoi(args[0])
				if err != nil {
					return err
				}
			}

			return InjectEvidence(
				cmd.Context(),
				rand.New(rand.NewSource(randomSeed)), //nolint: gosec
				cli.testnet,
				amount,
			)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Runs test cases against a running testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Test(cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "cleanup",
		Short: "Removes the testnet directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Cleanup(cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "logs",
		Short: "Shows the testnet logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execComposeVerbose(cli.testnet.Dir, "logs")
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "tail",
		Short: "Tails the testnet logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return execComposeVerbose(cli.testnet.Dir, "logs", "--follow")
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "benchmark",
		Short: "Benchmarks testnet",
		Long: `Benchmarks the following metrics:
	Mean Block Interval
	Standard Deviation
	Min Block Interval
	Max Block Interval
over a 100 block sampling period.
		
Does not run any perturbations.
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := Cleanup(cli.testnet); err != nil {
				return err
			}
			if err := Setup(cli.testnet, cli.infp); err != nil {
				return err
			}

			chLoadResult := make(chan error)
			ctx, loadCancel := context.WithCancel(context.Background())
			defer loadCancel()
			go func() {
				err := Load(ctx, cli.testnet)
				if err != nil {
					logger.Error(fmt.Sprintf("Transaction load errored: %v", err.Error()))
				}
				chLoadResult <- err
			}()

			if err := Start(cli.testnet); err != nil {
				return err
			}

			if err := Wait(cli.testnet, 5); err != nil { // allow some txs to go through
				return err
			}

			// we benchmark performance over the next 100 blocks
			if err := Benchmark(cli.testnet, 100); err != nil {
				return err
			}

			loadCancel()
			if err := <-chLoadResult; err != nil {
				return err
			}

			if err := Cleanup(cli.testnet); err != nil {
				return err
			}

			return nil
		},
	})

	return cli
}

// Run runs the CLI.
func (cli *CLI) Run() {
	if err := cli.root.Execute(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
