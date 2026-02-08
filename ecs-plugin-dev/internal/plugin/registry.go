package plugin

import (
	"fmt"
	"sync"

	"ecs-plugin-dev/internal/strategy"
)

// Registry manages deployment strategies
type Registry struct {
	mu         sync.RWMutex
	strategies map[string]strategy.Strategy
}

// NewRegistry creates a new strategy registry
func NewRegistry() *Registry {
	return &Registry{
		strategies: make(map[string]strategy.Strategy),
	}
}

// Register adds a strategy to the registry
func (r *Registry) Register(name string, s strategy.Strategy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.strategies[name]; exists {
		return fmt.Errorf("strategy %s already registered", name)
	}

	r.strategies[name] = s
	return nil
}

// Get retrieves a strategy by name
func (r *Registry) Get(name string) (strategy.Strategy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.strategies[name]
	return s, ok
}

// List returns all registered strategy names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		names = append(names, name)
	}
	return names
}

// Unregister removes a strategy from the registry
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.strategies, name)
}
