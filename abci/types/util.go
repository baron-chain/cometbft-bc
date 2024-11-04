package types

import "sort"

// ValidatorUpdates represents a sortable list of validator updates.
// It implements sort.Interface for deterministic ordering.
type ValidatorUpdates []ValidatorUpdate

// Ensure ValidatorUpdates implements sort.Interface at compile time
var _ sort.Interface = (ValidatorUpdates)(nil)

// Len returns the number of validators in the update list.
func (v ValidatorUpdates) Len() int {
	return len(v)
}

// Less compares validators by their public keys.
// Returns true if validator at index i should be ordered before validator at index j.
func (v ValidatorUpdates) Less(i, j int) bool {
	return v[i].PubKey.Compare(v[j].PubKey) <= 0
}

// Swap exchanges validators at indices i and j.
func (v ValidatorUpdates) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}
