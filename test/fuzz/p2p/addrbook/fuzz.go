package addr

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/p2p/pex"
)

const (
	addrBookPath = "./testdata/addrbook.json"
	maxBias      = 100
)

var (
	// Error definitions
	ErrNilAddress    = errors.New("nil address returned from pick")
	ErrInvalidFormat = errors.New("invalid address format")
	ErrAddressAdd    = errors.New("failed to add address")

	// Global state
	testState struct {
		sync.Once
		addrBook *pex.AddrBook
		err      error
	}
)

// FuzzResult represents possible fuzzing outcomes
type FuzzResult int

const (
	FuzzError     FuzzResult = -1 // Invalid input
	FuzzIgnore    FuzzResult = 0  // Valid but uninteresting input
	FuzzInterest  FuzzResult = 1  // Interesting input
)

// initializeAddrBook ensures thread-safe singleton initialization of the address book
func initializeAddrBook() error {
	testState.Do(func() {
		testState.addrBook = pex.NewAddrBook(addrBookPath, true)
		if testState.addrBook == nil {
			testState.err = errors.New("failed to create address book")
		}
	})
	return testState.err
}

// validateAddress attempts to unmarshal and validate a network address
func validateAddress(data []byte) (*p2p.NetAddress, error) {
	addr := new(p2p.NetAddress)
	if err := json.Unmarshal(data, addr); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFormat, err)
	}

	// Basic address validation
	if addr == nil || addr.ID == "" || addr.IP == nil {
		return nil, fmt.Errorf("%w: missing required fields", ErrInvalidFormat)
	}

	return addr, nil
}

// addAddressToBook attempts to add an address to the address book
func addAddressToBook(book *pex.AddrBook, addr *p2p.NetAddress) error {
	if err := book.AddAddress(addr, addr); err != nil {
		return fmt.Errorf("%w: %v", ErrAddressAdd, err)
	}
	return nil
}

// pickAddressFromBook attempts to pick a random address from the book
func pickAddressFromBook(book *pex.AddrBook) (*p2p.NetAddress, error) {
	bias := rand.Intn(maxBias)
	addr := book.PickAddress(bias)
	if addr == nil {
		return nil, fmt.Errorf("%w: bias=%d, book_size=%d",
			ErrNilAddress, bias, book.Size())
	}
	return addr, nil
}

// Fuzz implements the fuzzing entry point for address book testing
func Fuzz(data []byte) int {
	// Initialize address book
	if err := initializeAddrBook(); err != nil {
		return int(FuzzError)
	}

	// Validate input address
	addr, err := validateAddress(data)
	if err != nil {
		// Invalid format is an expected error
		if errors.Is(err, ErrInvalidFormat) {
			return int(FuzzIgnore)
		}
		return int(FuzzError)
	}

	// Attempt to add address to book
	if err := addAddressToBook(testState.addrBook, addr); err != nil {
		// Expected errors during normal operation
		if errors.Is(err, ErrAddressAdd) {
			return int(FuzzIgnore)
		}
		return int(FuzzError)
	}

	// Verify address picking functionality
	_, err = pickAddressFromBook(testState.addrBook)
	if err != nil {
		// This should never happen in normal operation
		panic(fmt.Sprintf("critical error in address picking: %v", err))
	}

	return int(FuzzInterest)
}
