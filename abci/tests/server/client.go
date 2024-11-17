package testsuite

import (
    "bytes"
    "errors"
    "fmt"
    
    abcicli "github.com/baron-chain/cometbft-bc/abci/client"
    "github.com/baron-chain/cometbft-bc/abci/types"
    bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
)

var (
    ErrInitChain        = errors.New("baron chain init chain failed")
    ErrCommit          = errors.New("baron chain commit failed")
    ErrDeliverTx       = errors.New("baron chain delivery failed")
    ErrCheckTx         = errors.New("baron chain validation failed")
    ErrPrepareProposal = errors.New("baron chain proposal preparation failed")
    ErrProcessProposal = errors.New("baron chain proposal processing failed")
)

// TestResult represents a test case result
type TestResult struct {
    Success bool
    Error   error
    Message string
}

// InitChain initializes the Baron Chain validator set
func InitChain(client abcicli.Client) TestResult {
    // Generate random validator set
    validators := generateValidators(10)
    
    resp, err := client.InitChainSync(types.RequestInitChain{
        Validators: validators,
    })

    if err != nil {
        return TestResult{
            Success: false,
            Error:   fmt.Errorf("%w: %v", ErrInitChain, err),
            Message: "validator initialization failed",
        }
    }

    return TestResult{
        Success: true,
        Message: "validator set initialized successfully",
    }
}

// Commit verifies Baron Chain block commitment
func Commit(client abcicli.Client, expectedHash []byte) TestResult {
    resp, err := client.CommitSync()
    if err != nil {
        return TestResult{
            Success: false,
            Error:   fmt.Errorf("%w: %v", ErrCommit, err),
            Message: "commit failed",
        }
    }

    if !bytes.Equal(resp.Data, expectedHash) {
        return TestResult{
            Success: false,
            Error:   ErrCommit,
            Message: fmt.Sprintf("hash mismatch - got: %X, want: %X", resp.Data, expectedHash),
        }
    }

    return TestResult{
        Success: true,
        Message: "block committed successfully",
    }
}

// DeliverTx tests Baron Chain transaction delivery
func DeliverTx(client abcicli.Client, tx []byte, expectCode uint32, expectData []byte) TestResult {
    resp, err := client.DeliverTxSync(types.RequestDeliverTx{Tx: tx})
    if err != nil {
        return TestResult{
            Success: false,
            Error:   fmt.Errorf("%w: %v", ErrDeliverTx, err),
            Message: "delivery failed",
        }
    }

    if resp.Code != expectCode {
        return TestResult{
            Success: false,
            Error:   ErrDeliverTx,
            Message: fmt.Sprintf("code mismatch - got: %d, want: %d, log: %s", 
                               resp.Code, expectCode, resp.Log),
        }
    }

    if !bytes.Equal(resp.Data, expectData) {
        return TestResult{
            Success: false,
            Error:   ErrDeliverTx,
            Message: fmt.Sprintf("data mismatch - got: %X, want: %X", resp.Data, expectData),
        }
    }

    return TestResult{
        Success: true,
        Message: "transaction delivered successfully",
    }
}

// PrepareProposal tests Baron Chain proposal preparation
func PrepareProposal(client abcicli.Client, txs [][]byte, expectedTxs [][]byte, expectData []byte) TestResult {
    resp, err := client.PrepareProposalSync(types.RequestPrepareProposal{Txs: txs})
    if err != nil {
        return TestResult{
            Success: false,
            Error:   fmt.Errorf("%w: %v", ErrPrepareProposal, err),
            Message: "proposal preparation failed",
        }
    }

    for i, tx := range resp.Txs {
        if !bytes.Equal(tx, expectedTxs[i]) {
            return TestResult{
                Success: false,
                Error:   ErrPrepareProposal,
                Message: fmt.Sprintf("tx mismatch at index %d - got: %X, want: %X", i, tx, expectedTxs[i]),
            }
        }
    }

    return TestResult{
        Success: true,
        Message: "proposal prepared successfully",
    }
}

// ProcessProposal tests Baron Chain proposal processing
func ProcessProposal(client abcicli.Client, txs [][]byte, expectStatus types.ResponseProcessProposal_ProposalStatus) TestResult {
    resp, err := client.ProcessProposalSync(types.RequestProcessProposal{Txs: txs})
    if err != nil {
        return TestResult{
            Success: false,
            Error:   fmt.Errorf("%w: %v", ErrProcessProposal, err),
            Message: "proposal processing failed",
        }
    }

    if resp.Status != expectStatus {
        return TestResult{
            Success: false,
            Error:   ErrProcessProposal,
            Message: fmt.Sprintf("status mismatch - got: %v, want: %v", resp.Status, expectStatus),
        }
    }

    return TestResult{
        Success: true,
        Message: "proposal processed successfully",
    }
}

// CheckTx tests Baron Chain transaction validation
func CheckTx(client abcicli.Client, tx []byte, expectCode uint32, expectData []byte) TestResult {
    resp, err := client.CheckTxSync(types.RequestCheckTx{Tx: tx})
    if err != nil {
        return TestResult{
            Success: false,
            Error:   fmt.Errorf("%w: %v", ErrCheckTx, err),
            Message: "validation failed",
        }
    }

    if resp.Code != expectCode {
        return TestResult{
            Success: false,
            Error:   ErrCheckTx,
            Message: fmt.Sprintf("code mismatch - got: %d, want: %d, log: %s", 
                               resp.Code, expectCode, resp.Log),
        }
    }

    if !bytes.Equal(resp.Data, expectData) {
        return TestResult{
            Success: false,
            Error:   ErrCheckTx,
            Message: fmt.Sprintf("data mismatch - got: %X, want: %X", resp.Data, expectData),
        }
    }

    return TestResult{
        Success: true,
        Message: "transaction validated successfully",
    }
}

// Helper function to generate random validator set
func generateValidators(count int) []types.ValidatorUpdate {
    validators := make([]types.ValidatorUpdate, count)
    for i := 0; i < count; i++ {
        pubkey := bcrand.Bytes(33)
        power := bcrand.Int()
        validators[i] = types.UpdateValidator(pubkey, int64(power), "")
    }
    return validators
}
