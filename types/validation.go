package types

import (
    "fmt"
    "time"
    "github.com/baron-chain/cometbft-bc/crypto/tmhash"
    bctime "github.com/baron-chain/cometbft-bc/types/time"
)

const (
    maxTimeDeviation = 8766 * time.Hour // One year in hours
    minHashSize     = 1
)

// ValidateTime ensures timestamps are within acceptable range for Baron Chain
// Prevents time-based attacks while maintaining network synchronization
func ValidateTime(t time.Time) error {
    now := bctime.Now()
    timeDiff := t.Sub(now)
    
    if abs := absoluteDuration(timeDiff); abs > maxTimeDeviation {
        return fmt.Errorf("time deviation exceeds limits: got %v, max allowed: ±%v",
            timeDiff, maxTimeDeviation)
    }
    
    return nil
}

// ValidateHash ensures hash sizes meet Baron Chain's quantum-safe requirements
func ValidateHash(h []byte) error {
    hashLen := len(h)
    
    if hashLen == 0 {
        return nil
    }
    
    if hashLen != tmhash.Size {
        return fmt.Errorf("invalid hash size: expected %d bytes, got %d bytes",
            tmhash.Size, hashLen)
    }
    
    return nil
}

// absoluteDuration returns the absolute value of a time.Duration
func absoluteDuration(d time.Duration) time.Duration {
    if d < 0 {
        return -d
    }
    return d
}

// ValidateTimeAndHash combines time and hash validation for atomic operations
func ValidateTimeAndHash(t time.Time, h []byte) error {
    if err := ValidateTime(t); err != nil {
        return fmt.Errorf("time validation failed: %w", err)
    }
    
    if err := ValidateHash(h); err != nil {
        return fmt.Errorf("hash validation failed: %w", err)
    }
    
    return nil
}
