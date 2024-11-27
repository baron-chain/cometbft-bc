package types

import (
    "fmt"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
)

func TestQueryTxFor(t *testing.T) {
    t.Run("basic transaction query", func(t *testing.T) {
        tx := Tx("test_transaction")
        expected := fmt.Sprintf("tm.event='Tx' AND tx.hash='%X'", tx.Hash())
        actual := EventQueryTxFor(tx).String()
        
        assert.Equal(t, expected, actual)
    })

    t.Run("quantum-safe transaction query", func(t *testing.T) {
        // Generate PQC keys for testing
        privKey, pubKey := kyber.GenerateKeypair()
        
        // Create and sign quantum-safe transaction
        tx := NewPQCTx("test_quantum_tx", privKey)
        query := EventQueryTxFor(tx).String()
        
        // Verify query contains quantum signature
        require.Contains(t, query, "tx.pqc_sig")
        require.True(t, kyber.Verify(pubKey, tx.Hash()))
    })
}

func TestQueryForEvent(t *testing.T) {
    testCases := []struct {
        name     string
        event    string
        expected string
    }{
        {
            name:     "new block event",
            event:    EventNewBlock,
            expected: "tm.event='NewBlock'",
        },
        {
            name:     "new evidence event", 
            event:    EventNewEvidence,
            expected: "tm.event='NewEvidence'",
        },
        {
            name:     "quantum validation event",
            event:    EventPQCValidation,
            expected: "tm.event='PQCValidation'",
        },
        {
            name:     "sidechain event",
            event:    EventSidechain,
            expected: "tm.event='Sidechain'",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            query := QueryForEvent(tc.event).String()
            assert.Equal(t, tc.expected, query)
        })
    }
}

func TestAIOptimizedQueries(t *testing.T) {
    t.Run("AI routing query", func(t *testing.T) {
        query := QueryForEvent(EventAIRouting).String()
        assert.Equal(t, "tm.event='AIRouting'", query)
    })

    t.Run("AI validator selection", func(t *testing.T) {
        query := QueryForEvent(EventAIValidatorUpdate).String()
        assert.Equal(t, "tm.event='AIValidatorUpdate'", query)
    })
}

func TestQuantumSafeEventQueries(t *testing.T) {
    t.Run("PQC key rotation event", func(t *testing.T) {
        query := QueryForEvent(EventPQCKeyRotation).String()
        assert.Equal(t, "tm.event='PQCKeyRotation'", query)
    })

    t.Run("quantum threat detection", func(t *testing.T) {
        query := QueryForEvent(EventQuantumThreatDetected).String()
        assert.Equal(t, "tm.event='QuantumThreatDetected'", query)
    })
}
