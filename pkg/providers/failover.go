package providers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// FailoverReason classifies why an LLM request failed for fallback decisions.
type FailoverReason string

const (
	FailoverReasonAuth       FailoverReason = "auth"
	FailoverReasonRateLimit  FailoverReason = "rate_limit"
	FailoverReasonBilling    FailoverReason = "billing"
	FailoverReasonTimeout    FailoverReason = "timeout"
	FailoverReasonFormat     FailoverReason = "format"
	FailoverReasonOverloaded FailoverReason = "overloaded"
	FailoverReasonUnknown    FailoverReason = "unknown"
)

// FailoverError wraps an LLM provider error with classification metadata.
type FailoverError struct {
	Reason   FailoverReason
	Provider string
	Model    string
	Status   int
	Wrapped  error
}

func (e *FailoverError) Error() string {
	return fmt.Sprintf("failover(%s): provider=%s model=%s status=%d: %v",
		e.Reason, e.Provider, e.Model, e.Status, e.Wrapped)
}

func (e *FailoverError) Unwrap() error {
	return e.Wrapped
}

// IsRetriable returns true if this error should trigger fallback to next candidate.
// Non-retriable: Format errors (bad request structure, image dimension/size).
func (e *FailoverError) IsRetriable() bool {
	return e.Reason != FailoverReasonFormat
}

// errorPattern defines a single pattern (string or regex) for error classification.
type errorPattern struct {
	substring string
	regex     *regexp.Regexp
}

func substr(s string) errorPattern { return errorPattern{substring: s} }
func rxp(r string) errorPattern    { return errorPattern{regex: regexp.MustCompile("(?i)" + r)} }

// Error patterns organized by FailoverReason (~40 patterns from production).
var (
	rateLimitPatterns = []errorPattern{
		rxp(`rate[_ ]limit`),
		substr("too many requests"),
		substr("429"),
		substr("exceeded your current quota"),
		rxp(`exceeded.*quota`),
		rxp(`resource has been exhausted`),
		rxp(`resource.*exhausted`),
		substr("resource_exhausted"),
		substr("quota exceeded"),
		substr("usage limit"),
	}

	overloadedPatterns = []errorPattern{
		rxp(`overloaded_error`),
		rxp(`"type"\s*:\s*"overloaded_error"`),
		substr("overloaded"),
	}

	timeoutPatterns = []errorPattern{
		substr("timeout"),
		substr("timed out"),
		substr("deadline exceeded"),
		substr("context deadline exceeded"),
		substr("connection refused"),
		substr("no such host"),
		substr("dial tcp"),
	}

	billingPatterns = []errorPattern{
		rxp(`\b402\b`),
		substr("payment required"),
		substr("insufficient credits"),
		substr("credit balance"),
		substr("plans & billing"),
		substr("insufficient balance"),
	}

	authPatterns = []errorPattern{
		rxp(`invalid[_ ]?api[_ ]?key`),
		substr("incorrect api key"),
		substr("invalid token"),
		substr("authentication"),
		substr("re-authenticate"),
		substr("oauth token refresh failed"),
		substr("unauthorized"),
		substr("forbidden"),
		substr("access denied"),
		substr("expired"),
		substr("token has expired"),
		rxp(`\b401\b`),
		rxp(`\b403\b`),
		substr("no credentials found"),
		substr("no api key found"),
	}

	formatPatterns = []errorPattern{
		substr("string should match pattern"),
		substr("tool_use.id"),
		substr("tool_use_id"),
		substr("messages.1.content.1.tool_use.id"),
		substr("invalid request format"),
	}

	imageDimensionPatterns = []errorPattern{
		rxp(`image dimensions exceed max`),
	}

	imageSizePatterns = []errorPattern{
		rxp(`image exceeds.*mb`),
	}

	// Transient HTTP status codes that map to timeout (server-side failures).
	transientStatusCodes = map[int]bool{
		500: true, 502: true, 503: true,
		521: true, 522: true, 523: true, 524: true,
		529: true,
	}
)

// ClassifyError classifies an error into a FailoverError with reason.
// Returns nil if the error is not classifiable.
func ClassifyError(err error, provider, model string) *FailoverError {
	if err == nil {
		return nil
	}

	// Context cancellation: user abort, never fallback.
	if err == context.Canceled {
		return nil
	}

	// Context deadline exceeded: treat as timeout, always fallback.
	if err == context.DeadlineExceeded {
		return &FailoverError{
			Reason:   FailoverReasonTimeout,
			Provider: provider,
			Model:    model,
			Wrapped:  err,
		}
	}

	msg := strings.ToLower(err.Error())

	// Image dimension/size errors: non-retriable.
	if matchesAny(msg, imageDimensionPatterns) || matchesAny(msg, imageSizePatterns) {
		return &FailoverError{
			Reason:   FailoverReasonFormat,
			Provider: provider,
			Model:    model,
			Wrapped:  err,
		}
	}

	// Try HTTP status code extraction first.
	if status := extractHTTPStatus(msg); status > 0 {
		if reason := classifyByStatus(status); reason != "" {
			return &FailoverError{
				Reason:   reason,
				Provider: provider,
				Model:    model,
				Status:   status,
				Wrapped:  err,
			}
		}
	}

	// Message pattern matching (priority order).
	if reason := classifyByMessage(msg); reason != "" {
		return &FailoverError{
			Reason:   reason,
			Provider: provider,
			Model:    model,
			Wrapped:  err,
		}
	}

	return nil
}

// ClassifyErrorSimple provides backward-compatible classification with status code.
func ClassifyErrorSimple(err error, statusCode int) ErrorClassification {
	if err == nil {
		return ErrorClassification{
			Reason:    FailoverReasonUnknown,
			Retriable: false,
			Message:   "no error",
		}
	}

	failErr := ClassifyError(err, "", "")
	if failErr != nil {
		return ErrorClassification{
			Reason:    failErr.Reason,
			Retriable: failErr.IsRetriable(),
			Message:   err.Error(),
		}
	}

	// Fall back to status code classification if pattern matching failed.
	if statusCode > 0 {
		if reason := classifyByStatus(statusCode); reason != "" {
			fe := &FailoverError{Reason: reason}
			return ErrorClassification{
				Reason:    reason,
				Retriable: fe.IsRetriable(),
				Message:   err.Error(),
			}
		}
	}

	return ErrorClassification{
		Reason:    FailoverReasonUnknown,
		Retriable: false,
		Message:   err.Error(),
	}
}

// ErrorClassification contains error classification details.
type ErrorClassification struct {
	Reason    FailoverReason
	Retriable bool
	Message   string
}

// classifyByStatus maps HTTP status codes to FailoverReason.
func classifyByStatus(status int) FailoverReason {
	switch {
	case status == 401 || status == 403:
		return FailoverReasonAuth
	case status == 402:
		return FailoverReasonBilling
	case status == 408:
		return FailoverReasonTimeout
	case status == 429:
		return FailoverReasonRateLimit
	case status == 400:
		return FailoverReasonFormat
	case transientStatusCodes[status]:
		return FailoverReasonTimeout
	}
	return ""
}

// classifyByMessage matches error messages against patterns.
// Priority order matters.
func classifyByMessage(msg string) FailoverReason {
	if matchesAny(msg, rateLimitPatterns) {
		return FailoverReasonRateLimit
	}
	if matchesAny(msg, overloadedPatterns) {
		return FailoverReasonRateLimit // Overloaded treated as rate_limit
	}
	if matchesAny(msg, billingPatterns) {
		return FailoverReasonBilling
	}
	if matchesAny(msg, timeoutPatterns) {
		return FailoverReasonTimeout
	}
	if matchesAny(msg, authPatterns) {
		return FailoverReasonAuth
	}
	if matchesAny(msg, formatPatterns) {
		return FailoverReasonFormat
	}
	return ""
}

// extractHTTPStatus extracts an HTTP status code from an error message.
func extractHTTPStatus(msg string) int {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`status[:\s]+(\d{3})`),
		regexp.MustCompile(`HTTP[/\s]+\d*\.?\d*\s+(\d{3})`),
	}

	for _, p := range patterns {
		if m := p.FindStringSubmatch(msg); len(m) > 1 {
			return parseDigits(m[1])
		}
	}

	return 0
}

// matchesAny checks if msg matches any of the patterns.
func matchesAny(msg string, patterns []errorPattern) bool {
	for _, p := range patterns {
		if p.regex != nil {
			if p.regex.MatchString(msg) {
				return true
			}
		} else if p.substring != "" {
			if strings.Contains(msg, p.substring) {
				return true
			}
		}
	}
	return false
}

// parseDigits converts a string of digits to an int.
func parseDigits(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
