package client

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsSessionExpired(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &APIError{ErrCode: errCodeSessionExpired})
	if !IsSessionExpired(err) {
		t.Fatalf("expected wrapped session-expired error to be detected")
	}
}

func TestIsSessionExpiredFalseForOtherErrors(t *testing.T) {
	if IsSessionExpired(errors.New("plain error")) {
		t.Fatalf("expected non-API error to be rejected")
	}
	if IsSessionExpired(&APIError{ErrCode: 123}) {
		t.Fatalf("expected different API error code to be rejected")
	}
}
