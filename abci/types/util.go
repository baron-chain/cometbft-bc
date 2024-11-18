package types

import (
    "sort"
    "sync"
)

// ValidatorUpdates represents a sortable slice of validator updates
type ValidatorUpdates []ValidatorUpdate

var validatorUpdatePool = sync.Pool{
    New: func() interface{} {
        return make(ValidatorUpdates, 0)
    },
}

// NewValidatorUpdates creates a new ValidatorUpdates instance
func NewValidatorUpdates(capacity int) ValidatorUpdates {
    if capacity <= 0 {
        return validatorUpdatePool.Get().(ValidatorUpdates)
    }
    return make(ValidatorUpdates, 0, capacity)
}

// ReleaseValidatorUpdates returns the updates to the pool
func ReleaseValidatorUpdates(v ValidatorUpdates) {
    if cap(v) > 1000 { // Don't pool large slices
        return
    }
    v = v[:0]
    validatorUpdatePool.Put(v)
}

// Sorting interface implementation
func (v ValidatorUpdates) Len() int { 
    return len(v) 
}

func (v ValidatorUpdates) Less(i, j int) bool {
    return v[i].PubKey.Compare(v[j].PubKey) <= 0
}

func (v ValidatorUpdates) Swap(i, j int) {
    v[i], v[j] = v[j], v[i]
}

// Sort provides a convenient way to sort validator updates
func (v ValidatorUpdates) Sort() {
    if len(v) > 1 {
        sort.Sort(v)
    }
}

// Add appends a validator update to the list
func (v *ValidatorUpdates) Add(update ValidatorUpdate) {
    *v = append(*v, update)
}

// AddBatch adds multiple validator updates efficiently
func (v *ValidatorUpdates) AddBatch(updates ...ValidatorUpdate) {
    if len(updates) == 0 {
        return
    }
    *v = append(*v, updates...)
}

// Remove removes a validator update by public key
func (v *ValidatorUpdates) Remove(pubKey PubKey) bool {
    for i := range *v {
        if (*v)[i].PubKey.Equal(pubKey) {
            *v = append((*v)[:i], (*v)[i+1:]...)
            return true
        }
    }
    return false
}

// Clear removes all validator updates
func (v *ValidatorUpdates) Clear() {
    *v = (*v)[:0]
}

// Clone creates a deep copy of ValidatorUpdates
func (v ValidatorUpdates) Clone() ValidatorUpdates {
    if v == nil {
        return nil
    }
    clone := make(ValidatorUpdates, len(v))
    copy(clone, v)
    return clone
}

// Equal checks if two ValidatorUpdates are identical
func (v ValidatorUpdates) Equal(other ValidatorUpdates) bool {
    if len(v) != len(other) {
        return false
    }
    
    // Both slices must be sorted for comparison
    v.Sort()
    other.Sort()
    
    for i := range v {
        if !v[i].PubKey.Equal(other[i].PubKey) || v[i].Power != other[i].Power {
            return false
        }
    }
    return true
}

// Contains checks if a validator with given public key exists
func (v ValidatorUpdates) Contains(pubKey PubKey) bool {
    for _, update := range v {
        if update.PubKey.Equal(pubKey) {
            return true
        }
    }
    return false
}

// Filter returns a new ValidatorUpdates containing only elements that satisfy the predicate
func (v ValidatorUpdates) Filter(predicate func(ValidatorUpdate) bool) ValidatorUpdates {
    result := NewValidatorUpdates(0)
    for _, update := range v {
        if predicate(update) {
            result = append(result, update)
        }
    }
    return result
}

// Ensure ValidatorUpdates implements sort.Interface
var _ sort.Interface = (ValidatorUpdates)(nil)
