package main

import "testing"

func TestCronCommand_RegistersRunSubcommand(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"cron", "run", "job-123"})
	if err != nil {
		t.Fatalf("find cron run command: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	if got, want := cmd.Name(), "run"; got != want {
		t.Fatalf("expected command name %q, got %q", want, got)
	}
	if got, want := cmd.Parent(), cronCmd; got != want {
		t.Fatalf("expected parent command %q, got %q", want.Name(), got.Name())
	}
}

func TestCronRunCommand_RequiresExactlyOneArg(t *testing.T) {
	if err := cronRunCmd.Args(cronRunCmd, []string{}); err == nil {
		t.Fatal("expected args validation error for empty args")
	}
	if err := cronRunCmd.Args(cronRunCmd, []string{"job-123", "extra"}); err == nil {
		t.Fatal("expected args validation error for extra args")
	}
	if err := cronRunCmd.Args(cronRunCmd, []string{"job-123"}); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
}
