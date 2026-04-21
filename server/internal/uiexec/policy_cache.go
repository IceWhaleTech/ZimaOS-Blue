package uiexec

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type PolicyCache struct {
	mu       sync.RWMutex
	policies []Policy
}

func NewPolicyCache() *PolicyCache {
	return &PolicyCache{
		policies: make([]Policy, 0, 32),
	}
}

func (c *PolicyCache) Match(task string, state State) (Policy, bool) {
	if c == nil {
		return Policy{}, false
	}
	taskPattern := TaskPatternFromTask(task)
	statePattern := StatePatternFromState(state)

	c.mu.RLock()
	defer c.mu.RUnlock()
	// Fast path: exact match.
	for _, p := range c.policies {
		if p.TaskPattern == taskPattern && p.StatePattern == statePattern {
			return p, true
		}
	}

	bestScore := 0.0
	bestCost := 0
	var best Policy
	for _, p := range c.policies {
		taskSim := tokenOverlapScore(p.TaskPattern, taskPattern)
		if taskSim < 0.25 {
			continue
		}
		stateSim := 0.0
		if statePattern != "" && p.StatePattern != "" {
			stateSim = tokenOverlapScore(p.StatePattern, statePattern)
		}
		score := 0.65*taskSim + 0.35*stateSim
		cost := p.Cost
		if cost <= 0 {
			cost = len(p.Actions)
		}
		if score > bestScore || (score == bestScore && bestScore > 0 && cost > 0 && (bestCost == 0 || cost < bestCost)) {
			bestScore = score
			best = p
			bestCost = cost
		}
	}
	if bestScore >= 0.55 {
		return best, true
	}
	return Policy{}, false
}

// Upsert records a policy. Only successful runs participate in shortest-path learning.
func (c *PolicyCache) Upsert(policy Policy, success bool) {
	if c == nil || !success {
		return
	}
	if policy.Cost <= 0 {
		policy.Cost = len(policy.Actions)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.policies {
		if existing.TaskPattern != policy.TaskPattern || existing.StatePattern != policy.StatePattern {
			continue
		}
		if existing.Cost <= 0 {
			existing.Cost = len(existing.Actions)
		}
		if policy.Cost < existing.Cost {
			c.policies[i] = policy
		}
		return
	}
	c.policies = append(c.policies, policy)
}

func (c *PolicyCache) Save(path string) error {
	if c == nil {
		return errors.New("policy cache is nil")
	}
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return errors.New("policy cache path is required")
	}

	c.mu.RLock()
	payload := append([]Policy(nil), c.policies...)
	c.mu.RUnlock()

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *PolicyCache) Load(path string) error {
	if c == nil {
		return errors.New("policy cache is nil")
	}
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload []Policy
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	c.mu.Lock()
	c.policies = payload
	c.mu.Unlock()
	return nil
}
