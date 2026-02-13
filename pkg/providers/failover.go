// Package providers provides API failover and error classification.
package providers

import (
	"errors"
	"net/http"
	"strings"
)

// FailoverReason indicates why a failover occurred.
type FailoverReason string

const (
	// FailoverReasonAuth indicates authentication failure (401, 403, invalid API key).
	FailoverReasonAuth FailoverReason = "auth"

	// FailoverReasonRateLimit indicates rate limiting (429).
	FailoverReasonRateLimit FailoverReason = "rate_limit"

	// FailoverReasonBilling indicates billing/quota issues (payment required, quota exceeded).
	FailoverReasonBilling FailoverReason = "billing"

	// FailoverReasonNetwork indicates network connectivity issues.
	FailoverReasonNetwork FailoverReason = "network"

	// FailoverReasonServer indicates server errors (5xx).
	FailoverReasonServer FailoverReason = "server"

	// FailoverReasonUnknown indicates unknown error type.
	FailoverReasonUnknown FailoverReason = "unknown"
)

// ErrorClassification contains error classification details.
type ErrorClassification struct {
	Reason     FailoverReason
	ShouldCooldown bool
	Retriable  bool
	Message    string
}

// ClassifyError analyzes an error and determines the failover reason.
func ClassifyError(err error, statusCode int) ErrorClassification {
	if err == nil {
		return ErrorClassification{
			Reason:     FailoverReasonUnknown,
			ShouldCooldown: false,
			Retriable:  false,
			Message:    "no error",
		}
	}

	errMsg := strings.ToLower(err.Error())

	// Check HTTP status codes first
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorClassification{
			Reason:     FailoverReasonAuth,
			ShouldCooldown: true,
			Retriable:  true,
			Message:    "authentication failed",
		}

	case http.StatusTooManyRequests:
		return ErrorClassification{
			Reason:     FailoverReasonRateLimit,
			ShouldCooldown: true,
			Retriable:  true,
			Message:    "rate limit exceeded",
		}

	case http.StatusPaymentRequired, http.StatusUpgradeRequired:
		return ErrorClassification{
			Reason:     FailoverReasonBilling,
			ShouldCooldown: true,
			Retriable:  true,
			Message:    "billing or quota issue",
		}

	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ErrorClassification{
			Reason:     FailoverReasonServer,
			ShouldCooldown: false,
			Retriable:  true,
			Message:    "service temporarily unavailable",
		}
	}

	// Check for 5xx server errors
	if statusCode >= 500 && statusCode < 600 {
		return ErrorClassification{
			Reason:     FailoverReasonServer,
			ShouldCooldown: false,
			Retriable:  true,
			Message:    "server error",
		}
	}

	// Check error message content
	if containsAny(errMsg, []string{"invalid api key", "invalid_api_key", "unauthorized", "forbidden", "authentication"}) {
		return ErrorClassification{
			Reason:     FailoverReasonAuth,
			ShouldCooldown: true,
			Retriable:  true,
			Message:    "authentication error",
		}
	}

	if containsAny(errMsg, []string{"rate limit", "rate_limit", "too many requests"}) {
		return ErrorClassification{
			Reason:     FailoverReasonRateLimit,
			ShouldCooldown: true,
			Retriable:  true,
			Message:    "rate limit error",
		}
	}

	if containsAny(errMsg, []string{"quota", "billing", "payment", "insufficient", "credits"}) {
		return ErrorClassification{
			Reason:     FailoverReasonBilling,
			ShouldCooldown: true,
			Retriable:  true,
			Message:    "billing or quota error",
		}
	}

	if containsAny(errMsg, []string{"network", "connection", "timeout", "dial", "refused"}) {
		return ErrorClassification{
			Reason:     FailoverReasonNetwork,
			ShouldCooldown: false,
			Retriable:  true,
			Message:    "network error",
		}
	}

	// Check for specific Go error types
	if errors.Is(err, http.ErrHandlerTimeout) {
		return ErrorClassification{
			Reason:     FailoverReasonNetwork,
			ShouldCooldown: false,
			Retriable:  true,
			Message:    "request timeout",
		}
	}

	// Default: unknown error
	return ErrorClassification{
		Reason:     FailoverReasonUnknown,
		ShouldCooldown: false,
		Retriable:  false,
		Message:    err.Error(),
	}
}

// ShouldRetry determines if an error is retriable with another profile.
func ShouldRetry(classification ErrorClassification) bool {
	return classification.Retriable
}

// ShouldCooldown determines if the profile should be put on cooldown.
func ShouldCooldown(classification ErrorClassification) bool {
	return classification.ShouldCooldown
}

// containsAny checks if the string contains any of the substrings.
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
