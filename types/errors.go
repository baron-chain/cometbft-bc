package types
//BaronChain
import (
    "fmt"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
)

type (
    // CommitHeightError represents mismatch between expected and actual commit heights
    CommitHeightError struct {
        Expected int64
        Actual   int64 
    }

    // CommitSignatureError represents mismatch between expected and actual signature counts
    CommitSignatureError struct {
        Expected int
        Actual   int
    }

    // PQCError represents quantum cryptography related errors
    PQCError struct {
        Msg string
        Err error 
    }
)

func NewCommitHeightError(expected, actual int64) CommitHeightError {
    return CommitHeightError{
        Expected: expected,
        Actual:   actual,
    }
}

func (e CommitHeightError) Error() string {
    return fmt.Sprintf("commit height mismatch - expected: %d, got: %d", e.Expected, e.Actual)
}

func NewCommitSignatureError(expected, actual int) CommitSignatureError {
    return CommitSignatureError{
        Expected: expected,
        Actual:   actual,
    }
}

func (e CommitSignatureError) Error() string {
    return fmt.Sprintf("invalid signature count - expected: %d, got: %d", e.Expected, e.Actual) 
}

func NewPQCError(msg string, err error) PQCError {
    return PQCError{
        Msg: msg,
        Err: err,
    }
}

func (e PQCError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("PQC error: %s - %v", e.Msg, e.Err)
    }
    return fmt.Sprintf("PQC error: %s", e.Msg)
}

func VerifyPQCCommit(commit []byte, pubKey kyber.PublicKey) error {
    if !kyber.Verify(pubKey, commit) {
        return NewPQCError("failed to verify quantum signature", nil)
    }
    return nil
}
