package secretconnection

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/async"
	sc "github.com/cometbft/cometbft/p2p/conn"
)

var (
	ErrEmptyData        = errors.New("empty data provided")
	ErrConnInit         = errors.New("failed to initialize secret connection")
	ErrPubKeyMismatch   = errors.New("public key mismatch")
	ErrDataMismatch     = errors.New("data mismatch between written and read bytes")
	ErrConnectionAbort  = errors.New("connection setup aborted unexpectedly")
	ErrInvalidConnState = errors.New("invalid connection state")
)

// pipeConn implements a connection using io.Pipe
type pipeConn struct {
	*io.PipeReader
	*io.PipeWriter
}

// Close implements proper pipe connection cleanup
func (pc pipeConn) Close() error {
	var errs []error

	if err := pc.PipeWriter.CloseWithError(io.EOF); err != nil {
		errs = append(errs, fmt.Errorf("writer close error: %w", err))
	}

	if err := pc.PipeReader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("reader close error: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}
	return nil
}

// createConnPair creates a pair of pipe connections for testing
func createConnPair() (conn1, conn2 pipeConn) {
	reader2, writer1 := io.Pipe()
	reader1, writer2 := io.Pipe()
	return pipeConn{reader1, writer1}, pipeConn{reader2, writer2}
}

// connectionPair holds a pair of secret connections and their keys
type connectionPair struct {
	conn1, conn2     *sc.SecretConnection
	privKey1, privKey2 ed25519.PrivKey
}

// initSecretConnPair initializes a pair of secret connections for testing
func initSecretConnPair() (*connectionPair, error) {
	// Create base connections
	baseConn1, baseConn2 := createConnPair()

	// Generate key pairs
	privKey1 := ed25519.GenPrivKey()
	privKey2 := ed25519.GenPrivKey()
	pubKey1 := privKey1.PubKey()
	pubKey2 := privKey2.PubKey()

	var conn1, conn2 *sc.SecretConnection
	var setupError error

	// Initialize connections in parallel
	tasks, ok := async.Parallel(
		func(_ int) (interface{}, bool, error) {
			var err error
			conn1, err = sc.MakeSecretConnection(baseConn1, privKey1)
			if err != nil {
				return nil, true, fmt.Errorf("conn1 setup failed: %w", err)
			}

			if !conn1.RemotePubKey().Equals(pubKey2) {
				return nil, true, fmt.Errorf("%w: conn1 expected %v, got %v",
					ErrPubKeyMismatch, pubKey2, conn1.RemotePubKey())
			}
			return nil, false, nil
		},
		func(_ int) (interface{}, bool, error) {
			var err error
			conn2, err = sc.MakeSecretConnection(baseConn2, privKey2)
			if err != nil {
				return nil, true, fmt.Errorf("conn2 setup failed: %w", err)
			}

			if !conn2.RemotePubKey().Equals(pubKey1) {
				return nil, true, fmt.Errorf("%w: conn2 expected %v, got %v",
					ErrPubKeyMismatch, pubKey1, conn2.RemotePubKey())
			}
			return nil, false, nil
		},
	)

	if err := tasks.FirstError(); err != nil {
		setupError = err
	} else if !ok {
		setupError = ErrConnectionAbort
	}

	if setupError != nil {
		// Clean up resources on error
		if conn1 != nil {
			_ = baseConn1.Close()
		}
		if conn2 != nil {
			_ = baseConn2.Close()
		}
		return nil, fmt.Errorf("failed to initialize connection pair: %w", setupError)
	}

	return &connectionPair{
		conn1:    conn1,
		conn2:    conn2,
		privKey1: privKey1,
		privKey2: privKey2,
	}, nil
}

// verifyDataTransfer checks if data is correctly transferred between connections
func verifyDataTransfer(conn1, conn2 *sc.SecretConnection, data []byte) error {
	written, err := conn1.Write(data)
	if err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	readBuf := make([]byte, written)
	read, err := conn2.Read(readBuf)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	if !bytes.Equal(data[:written], readBuf[:read]) {
		return fmt.Errorf("%w: written %X != read %X",
			ErrDataMismatch, data[:written], readBuf[:read])
	}

	return nil
}

// Fuzz implements fuzzing for secret connection data transfer
func Fuzz(data []byte) int {
	if len(data) == 0 {
		return -1
	}

	pair, err := initSecretConnPair()
	if err != nil {
		return -1
	}

	if err := verifyDataTransfer(pair.conn1, pair.conn2, data); err != nil {
		// Return 0 for expected errors, -1 for unexpected ones
		if errors.Is(err, io.EOF) || errors.Is(err, ErrDataMismatch) {
			return 0
		}
		return -1
	}

	return 1
}
