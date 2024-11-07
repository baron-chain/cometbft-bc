package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/store"
	"github.com/cometbft/cometbft/test/loadtime/report"
)

type config struct {
	dbType string
	dataDir string
	csvOutput string
}

func parseFlags() *config {
	cfg := &config{}
	flag.StringVar(&cfg.dbType, "database-type", "goleveldb", "Database type for blockstore")
	flag.StringVar(&cfg.dataDir, "data-dir", "", "Path to CometBFT databases directory")
	flag.StringVar(&cfg.csvOutput, "csv", "", "Path for CSV output of latencies")
	flag.Parse()
	
	validateFlags(cfg)
	return cfg
}

func validateFlags(cfg *config) {
	if cfg.dbType == "" {
		log.Fatal("database-type is required")
	}
	if cfg.dataDir == "" {
		log.Fatal("data-dir is required")
	}
}

func expandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, strings.TrimPrefix(path, "~/")), nil
}

func initBlockstore(cfg *config) (*store.BlockStore, error) {
	expandedPath, err := expandPath(cfg.dataDir)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(expandedPath); err != nil {
		return nil, fmt.Errorf("invalid data directory: %w", err)
	}

	db, err := dbm.NewDB("blockstore", dbm.BackendType(cfg.dbType), expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	return store.NewBlockStore(db), nil
}

func writeCSV(path string, reports []report.Report) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	records := generateCSVRecords(reports)
	return writer.WriteAll(records)
}

func generateCSVRecords(reports []report.Report) [][]string {
	total := sumTotalRecords(reports)
	records := make([][]string, total+1)
	records[0] = []string{
		"experiment_id",
		"block_time",
		"duration_ns",
		"tx_hash",
		"connections",
		"rate",
		"size",
	}

	offset := 1
	for _, r := range reports {
		offset += writeReportRecords(records[offset:], r)
	}
	return records
}

func sumTotalRecords(reports []report.Report) int {
	total := 0
	for _, r := range reports {
		total += len(r.All)
	}
	return total
}

func writeReportRecords(records [][]string, r report.Report) int {
	idStr := r.ID.String()
	connStr := strconv.FormatInt(int64(r.Connections), 10)
	rateStr := strconv.FormatInt(int64(r.Rate), 10)
	sizeStr := strconv.FormatInt(int64(r.Size), 10)

	for i, v := range r.All {
		records[i] = []string{
			idStr,
			strconv.FormatInt(v.BlockTime.UnixNano(), 10),
			strconv.FormatInt(int64(v.Duration), 10),
			fmt.Sprintf("%X", v.Hash),
			connStr,
			rateStr,
			sizeStr,
		}
	}
	return len(r.All)
}

func printReportSummary(r report.Report) {
	fmt.Printf(`Experiment ID: %s

	Connections: %d
	Rate: %d
	Size: %d

	Total Valid Tx: %d
	Total Negative Latencies: %d
	Minimum Latency: %s
	Maximum Latency: %s
	Average Latency: %s
	Standard Deviation: %s

`, r.ID, r.Connections, r.Rate, r.Size,
		len(r.All), r.NegativeCount,
		r.Min, r.Max, r.Avg, r.StdDev)
}

func main() {
	cfg := parseFlags()

	blockstore, err := initBlockstore(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize blockstore: %v", err)
	}
	defer blockstore.Close()

	reports, err := report.GenerateFromBlockStore(blockstore)
	if err != nil {
		log.Fatalf("Failed to generate reports: %v", err)
	}

	if cfg.csvOutput != "" {
		if err := writeCSV(cfg.csvOutput, reports.List()); err != nil {
			log.Fatalf("Failed to write CSV: %v", err)
		}
		return
	}

	for _, r := range reports.List() {
		printReportSummary(r)
	}
	fmt.Printf("Total Invalid Tx: %d\n", reports.ErrorCount())
}
