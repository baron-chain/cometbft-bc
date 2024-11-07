package v1_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	mempoolv1 "github.com/cometbft/cometbft/test/fuzz/mempool/v1"
)

const (
	testdataCasesDir = "testdata/cases"
	panicErrMsg      = "testdata/cases test panic"
)

var (
	// Error definitions
	ErrReadDir     = errors.New("failed to read test cases directory")
	ErrOpenFile    = errors.New("failed to open test case file")
	ErrReadFile    = errors.New("failed to read test case file")
	ErrTestPanic   = errors.New("test case caused panic")
)

// TestCase represents a single fuzz test case
type TestCase struct {
	Name     string
	Path     string
	Content  []byte
}

// LoadTestCase loads a single test case from file
func LoadTestCase(dir, name string) (*TestCase, error) {
	path := filepath.Join(dir, name)
	
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrOpenFile, path, err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrReadFile, path, err)
	}

	return &TestCase{
		Name:    name,
		Path:    path,
		Content: content,
	}, nil
}

// LoadTestCases loads all test cases from the testdata directory
func LoadTestCases() ([]*TestCase, error) {
	entries, err := os.ReadDir(testdataCasesDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadDir, err)
	}

	var testCases []*TestCase
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}

		testCase, err := LoadTestCase(testdataCasesDir, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to load test case %s: %w", entry.Name(), err)
		}
		testCases = append(testCases, testCase)
	}

	return testCases, nil
}

// RunTestCase executes a single test case with panic recovery
func RunTestCase(t *testing.T, tc *TestCase) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: %v", panicErrMsg, r)
			t.FailNow()
		}
	}()

	// Run the fuzz test
	mempoolv1.Fuzz(tc.Content)
}

// TestMempoolTestdataCases runs all mempool fuzz test cases
func TestMempoolTestdataCases(t *testing.T) {
	testCases, err := LoadTestCases()
	require.NoError(t, err, "Failed to load test cases")
	require.NotEmpty(t, testCases, "No test cases found in %s", testdataCasesDir)

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel() // Run test cases in parallel
			RunTestCase(t, tc)
		})
	}
}
