package providers

import (
	"fmt"
	"sync"
)

// AdaptorFactory is a function that creates a new Adaptor instance.
type AdaptorFactory func() Adaptor

// Registry maintains a thread-safe registry of provider adaptors.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]AdaptorFactory
}

// globalRegistry is the default global provider registry.
var globalRegistry = NewRegistry()

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]AdaptorFactory),
	}
}

// Register registers a provider adaptor with the given name.
// The factory function will be called each time GetAdaptor is invoked.
func (r *Registry) Register(name string, factory AdaptorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get retrieves the factory for a registered provider.
func (r *Registry) Get(name string) (AdaptorFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.factories[name]
	return factory, exists
}

// List returns a list of all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// Unregister removes a provider from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factories, name)
}

// GetAdaptor creates a new Adaptor instance for the given provider name.
// It uses the global registry.
func GetAdaptor(name string) (Adaptor, error) {
	return globalRegistry.GetAdaptor(name)
}

// GetAdaptor creates a new Adaptor instance for the given provider name.
func (r *Registry) GetAdaptor(name string) (Adaptor, error) {
	factory, exists := r.Get(name)
	if !exists {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return factory(), nil
}

// Register registers a provider adaptor with the global registry.
func Register(name string, factory AdaptorFactory) {
	globalRegistry.Register(name, factory)
}

// List returns a list of all registered providers from the global registry.
func List() []string {
	return globalRegistry.List()
}

// Unregister removes a provider from the global registry.
func Unregister(name string) {
	globalRegistry.Unregister(name)
}
