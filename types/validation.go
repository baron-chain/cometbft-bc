package types

import (
   "fmt"
   "time"
   "github.com/cometbft/cometbft/crypto/tmhash"
   cmttime "github.com/cometbft/cometbft/types/time"
)

func ValidateTime(t time.Time) error {
   oneYear := 8766 * time.Hour
   now := cmttime.Now()
   
   if t.Before(now.Add(-oneYear)) || t.After(now.Add(oneYear)) {
       return fmt.Errorf("time drifted too much. Expected: -1 < %v < 1 year", now) 
   }
   return nil
}

func ValidateHash(h []byte) error {
   if len(h) > 0 && len(h) != tmhash.Size {
       return fmt.Errorf("expected size to be %d bytes, got %d bytes", tmhash.Size, len(h))
   }
   return nil
}
