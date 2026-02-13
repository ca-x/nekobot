package providers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LoadBalancer manages multiple providers with automatic failover.
// It uses sensible defaults for circuit breaker, timeouts, and retries.
type LoadBalancer struct {
	providers map[string]*ProviderState
	mu        sync.RWMutex
}

// ProviderState tracks the state of a provider.
type ProviderState struct {
	Name              string
	Client            *Client
	RequestCount      int64
	SuccessCount      int64
	FailureCount      int64
	ConsecutiveErrors int
	CircuitState      CircuitState
	LastFailure       time.Time
	LastSuccess       time.Time
	mu                sync.RWMutex
}

// CircuitState represents the circuit breaker state.
type CircuitState string

const (
	// CircuitClosed - normal operation
	CircuitClosed CircuitState = "closed"
	// CircuitOpen - provider unavailable
	CircuitOpen CircuitState = "open"
	// CircuitHalfOpen - testing if provider recovered
	CircuitHalfOpen CircuitState = "half_open"
)

// Sensible defaults
const (
	DefaultFailureThreshold = 5              // Open circuit after 5 consecutive failures
	DefaultSuccessThreshold = 2              // Close circuit after 2 consecutive successes
	DefaultCooldown         = 5 * time.Minute // Wait 5 minutes before retry
	DefaultTimeout          = 30 * time.Second
	DefaultLocalTimeout     = 60 * time.Second // Longer timeout for local providers
)

// NewLoadBalancer creates a new load balancer.
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		providers: make(map[string]*ProviderState),
	}
}

// RegisterProvider registers a provider with the load balancer.
func (lb *LoadBalancer) RegisterProvider(name string, client *Client) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	state := &ProviderState{
		Name:         name,
		Client:       client,
		CircuitState: CircuitClosed,
	}

	lb.providers[name] = state
	return nil
}

// Chat performs a chat request with automatic failover.
// Tries providers in order: primary, then fallback list.
func (lb *LoadBalancer) Chat(ctx context.Context, req *UnifiedRequest, providerOrder []string) (*UnifiedResponse, error) {
	var lastErr error

	for _, providerName := range providerOrder {
		state, err := lb.getProviderState(providerName)
		if err != nil {
			lastErr = err
			continue
		}

		// Check circuit breaker
		if !lb.canUseProvider(state) {
			lastErr = fmt.Errorf("provider %s circuit breaker is open", providerName)
			continue
		}

		// Try request with timeout
		reqCtx, cancel := lb.createContextWithTimeout(ctx, providerName)
		resp, err := lb.executeRequest(reqCtx, state, req)
		cancel()

		if err != nil {
			lastErr = err
			lb.recordFailure(state, err)
			continue
		}

		// Success
		lb.recordSuccess(state)
		return resp, nil
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// ChatStream performs a streaming chat request with automatic failover.
func (lb *LoadBalancer) ChatStream(ctx context.Context, req *UnifiedRequest, handler StreamHandler, providerOrder []string) error {
	var lastErr error

	for _, providerName := range providerOrder {
		state, err := lb.getProviderState(providerName)
		if err != nil {
			lastErr = err
			continue
		}

		// Check circuit breaker
		if !lb.canUseProvider(state) {
			lastErr = fmt.Errorf("provider %s circuit breaker is open", providerName)
			continue
		}

		// Try streaming request
		err = state.Client.ChatStream(ctx, req, handler)
		if err != nil {
			lastErr = err
			lb.recordFailure(state, err)
			continue
		}

		// Success
		lb.recordSuccess(state)
		return nil
	}

	return fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// createContextWithTimeout creates a context with appropriate timeout.
func (lb *LoadBalancer) createContextWithTimeout(ctx context.Context, providerName string) (context.Context, context.CancelFunc) {
	timeout := DefaultTimeout

	// Use longer timeout for local providers
	if isLocalProvider(providerName) {
		timeout = DefaultLocalTimeout
	}

	return context.WithTimeout(ctx, timeout)
}

// isLocalProvider checks if a provider is running locally.
func isLocalProvider(name string) bool {
	localProviders := map[string]bool{
		"ollama":   true,
		"lmstudio": true,
		"vllm":     true,
	}
	return localProviders[name]
}

// canUseProvider checks if a provider can be used based on circuit breaker state.
func (lb *LoadBalancer) canUseProvider(state *ProviderState) bool {
	state.mu.RLock()
	defer state.mu.RUnlock()

	switch state.CircuitState {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if cooldown period has passed
		if time.Since(state.LastFailure) > DefaultCooldown {
			// Transition to half-open
			state.mu.RUnlock()
			state.mu.Lock()
			state.CircuitState = CircuitHalfOpen
			state.mu.Unlock()
			state.mu.RLock()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// executeRequest executes a request with the given provider.
func (lb *LoadBalancer) executeRequest(ctx context.Context, state *ProviderState, req *UnifiedRequest) (*UnifiedResponse, error) {
	state.mu.Lock()
	state.RequestCount++
	state.mu.Unlock()

	return state.Client.Chat(ctx, req)
}

// recordSuccess records a successful request.
func (lb *LoadBalancer) recordSuccess(state *ProviderState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.SuccessCount++
	state.ConsecutiveErrors = 0
	state.LastSuccess = time.Now()

	// Check if we should close the circuit
	if state.CircuitState == CircuitHalfOpen {
		if state.SuccessCount >= int64(DefaultSuccessThreshold) {
			state.CircuitState = CircuitClosed
		}
	}
}

// recordFailure records a failed request.
func (lb *LoadBalancer) recordFailure(state *ProviderState, err error) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.FailureCount++
	state.ConsecutiveErrors++
	state.LastFailure = time.Now()

	// Check if we should open the circuit
	if state.ConsecutiveErrors >= DefaultFailureThreshold {
		state.CircuitState = CircuitOpen
	}
}

// getProviderState retrieves a provider state by name.
func (lb *LoadBalancer) getProviderState(name string) (*ProviderState, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	state, exists := lb.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", name)
	}

	return state, nil
}

// GetStats returns statistics for all providers.
func (lb *LoadBalancer) GetStats() map[string]ProviderStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	stats := make(map[string]ProviderStats)
	for name, state := range lb.providers {
		state.mu.RLock()
		stats[name] = ProviderStats{
			Name:              name,
			RequestCount:      state.RequestCount,
			SuccessCount:      state.SuccessCount,
			FailureCount:      state.FailureCount,
			ConsecutiveErrors: state.ConsecutiveErrors,
			CircuitState:      state.CircuitState,
			LastFailure:       state.LastFailure,
			LastSuccess:       state.LastSuccess,
		}
		state.mu.RUnlock()
	}

	return stats
}

// ProviderStats contains statistics for a provider.
type ProviderStats struct {
	Name              string
	RequestCount      int64
	SuccessCount      int64
	FailureCount      int64
	ConsecutiveErrors int
	CircuitState      CircuitState
	LastFailure       time.Time
	LastSuccess       time.Time
}
