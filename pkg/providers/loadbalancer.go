package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LoadBalancer manages multiple providers with automatic failover
// using exponential cooldown backoff.
type LoadBalancer struct {
	providers map[string]*ProviderState
	cooldown  *CooldownTracker
	mu        sync.RWMutex
}

// ProviderState tracks the state of a provider.
type ProviderState struct {
	Name         string
	Client       *Client
	RequestCount int64
	SuccessCount int64
	FailureCount int64
	LastFailure  time.Time
	LastSuccess  time.Time
	mu           sync.RWMutex
}

// Sensible defaults
const (
	DefaultTimeout      = 30 * time.Second
	DefaultLocalTimeout = 60 * time.Second // Longer timeout for local providers
)

// NewLoadBalancer creates a new load balancer with cooldown tracking.
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		providers: make(map[string]*ProviderState),
		cooldown:  NewCooldownTracker(),
	}
}

// RegisterProvider registers a provider with the load balancer.
func (lb *LoadBalancer) RegisterProvider(name string, client *Client) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	state := &ProviderState{
		Name:   name,
		Client: client,
	}

	lb.providers[name] = state
	return nil
}

// Chat performs a chat request with automatic failover.
// Tries providers in order, respecting cooldowns and error classification.
func (lb *LoadBalancer) Chat(ctx context.Context, req *UnifiedRequest, providerOrder []string) (*UnifiedResponse, error) {
	if len(providerOrder) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}

	var attempts []FallbackAttempt

	for i, providerName := range providerOrder {
		// Check context before each attempt.
		if ctx.Err() == context.Canceled {
			return nil, context.Canceled
		}

		// Check cooldown.
		if !lb.cooldown.IsAvailable(providerName) {
			remaining := lb.cooldown.CooldownRemaining(providerName)
			attempts = append(attempts, FallbackAttempt{
				Provider: providerName,
				Skipped:  true,
				Reason:   FailoverReasonRateLimit,
				Error: fmt.Errorf(
					"provider %s in cooldown (%s remaining)",
					providerName,
					remaining.Round(time.Second),
				),
			})
			continue
		}

		state, err := lb.getProviderState(providerName)
		if err != nil {
			attempts = append(attempts, FallbackAttempt{
				Provider: providerName,
				Error:    err,
			})
			continue
		}

		// Execute request with timeout.
		reqCtx, cancel := lb.createContextWithTimeout(ctx, providerName)
		start := time.Now()
		resp, err := lb.executeRequest(reqCtx, state, req)
		elapsed := time.Since(start)
		cancel()

		if err == nil {
			// Success.
			lb.cooldown.MarkSuccess(providerName)
			lb.recordSuccess(state)
			return resp, nil
		}

		// Context cancellation: abort immediately.
		if ctx.Err() == context.Canceled {
			attempts = append(attempts, FallbackAttempt{
				Provider: providerName,
				Error:    err,
				Duration: elapsed,
			})
			return nil, context.Canceled
		}

		// Classify the error.
		failErr := ClassifyError(err, providerName, req.Model)

		if failErr != nil && !failErr.IsRetriable() {
			// Non-retriable: abort immediately.
			lb.recordFailure(state)
			attempts = append(attempts, FallbackAttempt{
				Provider: providerName,
				Error:    failErr,
				Reason:   failErr.Reason,
				Duration: elapsed,
			})
			return nil, failErr
		}

		// Retriable error: mark failure and continue.
		reason := FailoverReasonUnknown
		if failErr != nil {
			reason = failErr.Reason
		}
		lb.cooldown.MarkFailure(providerName, reason)
		lb.recordFailure(state)
		attempts = append(attempts, FallbackAttempt{
			Provider: providerName,
			Error:    err,
			Reason:   reason,
			Duration: elapsed,
		})

		// If last candidate, return aggregate error.
		if i == len(providerOrder)-1 {
			return nil, &FallbackExhaustedError{Attempts: attempts}
		}
	}

	// All candidates were skipped (all in cooldown).
	return nil, &FallbackExhaustedError{Attempts: attempts}
}

// ChatStream performs a streaming chat request with automatic failover.
func (lb *LoadBalancer) ChatStream(ctx context.Context, req *UnifiedRequest, handler StreamHandler, providerOrder []string) error {
	if len(providerOrder) == 0 {
		return fmt.Errorf("no providers configured")
	}

	var attempts []FallbackAttempt

	for i, providerName := range providerOrder {
		if ctx.Err() == context.Canceled {
			return context.Canceled
		}

		if !lb.cooldown.IsAvailable(providerName) {
			remaining := lb.cooldown.CooldownRemaining(providerName)
			attempts = append(attempts, FallbackAttempt{
				Provider: providerName,
				Skipped:  true,
				Reason:   FailoverReasonRateLimit,
				Error: fmt.Errorf(
					"provider %s in cooldown (%s remaining)",
					providerName,
					remaining.Round(time.Second),
				),
			})
			continue
		}

		state, err := lb.getProviderState(providerName)
		if err != nil {
			attempts = append(attempts, FallbackAttempt{
				Provider: providerName,
				Error:    err,
			})
			continue
		}

		start := time.Now()
		err = state.Client.ChatStream(ctx, req, handler)
		elapsed := time.Since(start)

		if err == nil {
			lb.cooldown.MarkSuccess(providerName)
			lb.recordSuccess(state)
			return nil
		}

		if ctx.Err() == context.Canceled {
			attempts = append(attempts, FallbackAttempt{
				Provider: providerName,
				Error:    err,
				Duration: elapsed,
			})
			return context.Canceled
		}

		failErr := ClassifyError(err, providerName, req.Model)

		if failErr != nil && !failErr.IsRetriable() {
			lb.recordFailure(state)
			return failErr
		}

		reason := FailoverReasonUnknown
		if failErr != nil {
			reason = failErr.Reason
		}
		lb.cooldown.MarkFailure(providerName, reason)
		lb.recordFailure(state)
		attempts = append(attempts, FallbackAttempt{
			Provider: providerName,
			Error:    err,
			Reason:   reason,
			Duration: elapsed,
		})

		if i == len(providerOrder)-1 {
			return &FallbackExhaustedError{Attempts: attempts}
		}
	}

	return &FallbackExhaustedError{Attempts: attempts}
}

// FallbackAttempt records one attempt in the fallback chain.
type FallbackAttempt struct {
	Provider string
	Model    string
	Error    error
	Reason   FailoverReason
	Duration time.Duration
	Skipped  bool // true if skipped due to cooldown
}

// FallbackExhaustedError indicates all fallback candidates were tried and failed.
type FallbackExhaustedError struct {
	Attempts []FallbackAttempt
}

func (e *FallbackExhaustedError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("all %d providers failed:", len(e.Attempts)))
	for i, a := range e.Attempts {
		if a.Skipped {
			sb.WriteString(fmt.Sprintf("\n  [%d] %s: skipped (cooldown)", i+1, a.Provider))
		} else {
			sb.WriteString(fmt.Sprintf("\n  [%d] %s: %v (reason=%s, %s)",
				i+1, a.Provider, a.Error, a.Reason, a.Duration.Round(time.Millisecond)))
		}
	}
	return sb.String()
}

// createContextWithTimeout creates a context with appropriate timeout.
func (lb *LoadBalancer) createContextWithTimeout(ctx context.Context, providerName string) (context.Context, context.CancelFunc) {
	timeout := DefaultTimeout

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
	state.LastSuccess = time.Now()
}

// recordFailure records a failed request.
func (lb *LoadBalancer) recordFailure(state *ProviderState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.FailureCount++
	state.LastFailure = time.Now()
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

// GetCooldownTracker returns the cooldown tracker for external use.
func (lb *LoadBalancer) GetCooldownTracker() *CooldownTracker {
	return lb.cooldown
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
			LastFailure:       state.LastFailure,
			LastSuccess:       state.LastSuccess,
			CooldownRemaining: lb.cooldown.CooldownRemaining(name),
			ErrorCount:        lb.cooldown.ErrorCount(name),
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
	LastFailure       time.Time
	LastSuccess       time.Time
	CooldownRemaining time.Duration
	ErrorCount        int
}
