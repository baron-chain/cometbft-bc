package types

import (
	"sort"
	
	"github.com/baron-chain/cometbft-bc/crypto/rand"
	"github.com/baron-chain/cometbft-bc/libs/math"
)

type (
	Chooser interface {
		Choose(*rand.Rand) interface{}
	}

	ParamCombinations struct {
		params map[string][]interface{}
		keys   []string
	}

	UniformSelector []interface{}
	
	WeightedSelector map[interface{}]uint
	
	ProbabilitySelector map[string]float64
	
	UniformSetSelector []string
)

func NewParamCombinations(params map[string][]interface{}) *ParamCombinations {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	
	return &ParamCombinations{
		params: params,
		keys:   keys,
	}
}

func (pc *ParamCombinations) Generate() []map[string]interface{} {
	return pc.generateCombinations(make(map[string]interface{}), pc.keys)
}

func (pc *ParamCombinations) generateCombinations(
	current map[string]interface{}, 
	remaining []string,
) []map[string]interface{} {
	if len(remaining) == 0 {
		return []map[string]interface{}{current}
	}

	key := remaining[0]
	rest := remaining[1:]
	result := make([]map[string]interface{}, 0)

	for _, value := range pc.params[key] {
		next := make(map[string]interface{}, len(current)+1)
		for k, v := range current {
			next[k] = v
		}
		next[key] = value
		result = append(result, pc.generateCombinations(next, rest)...)
	}

	return result
}

func (us UniformSelector) Choose(r *rand.Rand) interface{} {
	if len(us) == 0 {
		return nil
	}
	return us[r.Intn(len(us))]
}

func (ws WeightedSelector) Choose(r *rand.Rand) interface{} {
	if len(ws) == 0 {
		return nil
	}

	total := uint(0)
	for _, weight := range ws {
		total += weight
	}

	if total == 0 {
		return nil
	}

	selected := r.Uint64() % uint64(total)
	var cumulative uint64

	for choice, weight := range ws {
		cumulative += uint64(weight)
		if selected < cumulative {
			return choice
		}
	}

	return nil
}

func (ps ProbabilitySelector) Choose(r *rand.Rand) []string {
	if len(ps) == 0 {
		return nil
	}

	selected := make([]string, 0, len(ps))
	for item, prob := range ps {
		if prob <= 0 || prob > 1 {
			continue
		}
		if r.Float64() <= prob {
			selected = append(selected, item)
		}
	}
	
	return selected
}

func (us UniformSetSelector) Choose(r *rand.Rand) []string {
	if len(us) == 0 {
		return nil
	}

	count := 1 + r.Intn(math.MaxInt(1, len(us)))
	indices := r.Perm(len(us))[:count]
	
	selected := make([]string, count)
	for i, idx := range indices {
		selected[i] = us[idx]
	}
	
	return selected
}
