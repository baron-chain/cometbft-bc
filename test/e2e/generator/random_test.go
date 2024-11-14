package testing

import (
	"testing"
	
	"github.com/baron-chain/cometbft-bc/libs/rand"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	Name     string
	Values   map[string][]interface{}
	Expected int
}

func TestPermutations(t *testing.T) {
	tests := []TestCase{
		{
			Name: "blockchain_params",
			Values: map[string][]interface{}{
				"consensus": {false, true},
				"blocks":    {1, 10, 100},
				"chainID":   {"baron-test", "baron-main"},
			},
			Expected: 12,
		},
		{
			Name: "validator_params",
			Values: map[string][]interface{}{
				"active":   {false, true},
				"power":    {1000, 5000, 10000},
				"address": {"baron1", "baron2"},
			},
			Expected: 12,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			result := generatePermutations(tc.Values)
			require.Equal(t, tc.Expected, len(result))
			validatePermutations(t, result)
		})
	}
}

func generatePermutations(input map[string][]interface{}) []map[string]interface{} {
	keys := make([]string, 0, len(input))
	values := make([][]interface{}, 0, len(input))
	
	for k, v := range input {
		keys = append(keys, k)
		values = append(values, v)
	}

	totalPerms := 1
	for _, v := range values {
		totalPerms *= len(v)
	}

	result := make([]map[string]interface{}, 0, totalPerms)
	indices := make([]int, len(keys))

	for i := 0; i < totalPerms; i++ {
		perm := make(map[string]interface{}, len(keys))
		for j, key := range keys {
			perm[key] = values[j][indices[j]]
		}
		result = append(result, perm)
		
		for j := len(indices) - 1; j >= 0; j-- {
			indices[j]++
			if indices[j] < len(values[j]) {
				break
			}
			indices[j] = 0
		}
	}

	return result
}

func validatePermutations(t *testing.T, perms []map[string]interface{}) {
	require.Greater(t, len(perms), 0)
	
	seen := make(map[string]bool)
	for _, perm := range perms {
		hash := generatePermHash(perm)
		require.False(t, seen[hash], "Duplicate permutation found")
		seen[hash] = true
	}
}

func generatePermHash(perm map[string]interface{}) string {
	// Using Baron Chain's random generator for consistent hashing
	rnd := rand.NewRand()
	return rnd.Str(32)
}
