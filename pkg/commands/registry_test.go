package commands

import (
	"strings"
	"testing"
)

func TestParseUnknownCommandDoesNotFallbackToHelp(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(&Command{Name: "help"}); err != nil {
		t.Fatalf("register help: %v", err)
	}

	name, args := reg.Parse("/not-real now")
	if name != "not-real" {
		t.Fatalf("expected unknown command name to be preserved, got %q", name)
	}
	if args != "now" {
		t.Fatalf("expected args to be preserved, got %q", args)
	}

	msg := reg.UnknownCommandMessage(name)
	if !strings.Contains(msg, "/not-real") {
		t.Fatalf("expected unknown command in message, got %q", msg)
	}
	if !strings.Contains(msg, "/help") {
		t.Fatalf("expected available command hint, got %q", msg)
	}
}

func TestIsCommandInterceptsMalformedSlash(t *testing.T) {
	reg := NewRegistry()

	if !reg.IsCommand("/") {
		t.Fatal("expected bare slash to be treated as malformed command input")
	}

	name, args := reg.Parse("/")
	if name != "" || args != "" {
		t.Fatalf("expected empty parsed command, got name=%q args=%q", name, args)
	}

	msg := MalformedCommandMessage()
	if !strings.Contains(msg, "/help") {
		t.Fatalf("expected help hint in malformed command message, got %q", msg)
	}
}

func TestParseNormalizesBotMentionCommand(t *testing.T) {
	reg := NewRegistry()

	name, args := reg.Parse("/Status@Nekobot   now")
	if name != "status" {
		t.Fatalf("expected command mention to normalize to status, got %q", name)
	}
	if args != "now" {
		t.Fatalf("expected args to be trimmed, got %q", args)
	}
}
