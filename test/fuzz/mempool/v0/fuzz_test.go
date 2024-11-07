package v0_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	mempoolv0 "github.com/cometbft/cometbft/test/fuzz/mempool/v0"
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

// LoadAllTestCases loads all test cases from the testdata directory
func LoadAllTestCases() ([]*TestCase, error) {
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
			require.Nilf(t, r, "testdata/cases test panic in %s", tc.Name)
			t.FailNow()
		}
	}()

	// Run the fuzz test
	mempoolv0.Fuzz(tc.Content)
}

// ValidateTestCases ensures we have valid test cases to run
func ValidateTestCases(t *testing.T, testCases []*TestCase) {
	t.Helper()
	require.NotEmpty(t, testCases, "No test cases found in %s", testdataCasesDir)
	
	for _, tc := range testCases {
		require.NotEmpty(t, tc.Name, "Test case has empty name")
		require.FileExists(t, tc.Path, "Test case file does not exist")
	}
}

// TestMempoolTestdataCases runs all mempool v0 fuzz test cases
func TestMempoolTestdataCases(t *testing.T) {
	// Load all test cases
	testCases, err := LoadAllTestCases()
	require.NoError(t, err, "Failed to load test cases")
	
	// Validate test cases
	ValidateTestCases(t, testCases)

	// Run test cases
	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel() // Run test cases in parallel
			RunTestCase(t, tc)
		})
	}
}
